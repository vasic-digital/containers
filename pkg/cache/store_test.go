package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func newSingleImageManifest(t *testing.T, id, url string, body []byte) (*Manifest, string) {
	t.Helper()
	hash := sha256Hex(body)
	m := &Manifest{
		Version: 1,
		Images: []ImageEntry{
			{ID: id, URL: url, SHA256: hash, Size: int64(len(body)), Format: "qcow2"},
		},
	}
	return m, hash
}

func newCountingHTTPServer(body []byte, hits *int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	}))
}

func TestStore_Get_HappyPath(t *testing.T) {
	body := []byte("fake qcow2 bytes")
	var hits int64
	srv := newCountingHTTPServer(body, &hits)
	defer srv.Close()

	cacheRoot := t.TempDir()
	m, hash := newSingleImageManifest(t, "alpine-x86_64", srv.URL, body)
	s := NewFilesystemStore(cacheRoot)

	path, err := s.Get(context.Background(), m, "alpine-x86_64")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(body) {
		t.Fatalf("blob bytes differ")
	}
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("expected 1 HTTP fetch, got %d", got)
	}
	// Second Get → cache hit; no new HTTP fetch
	if _, err := s.Get(context.Background(), m, "alpine-x86_64"); err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("cache hit failed; HTTP hits now %d (want 1)", got)
	}
	expected := filepath.Join(cacheRoot, "blobs", "sha256", hash)
	if path != expected {
		t.Fatalf("blob path: got %q want %q", path, expected)
	}
}

func TestStore_Get_SHA256Mismatch_RejectsAndRemovesBlob(t *testing.T) {
	body := []byte("real bytes")
	var hits int64
	srv := newCountingHTTPServer(body, &hits)
	defer srv.Close()

	cacheRoot := t.TempDir()
	// Manifest declares the WRONG SHA — server returns body whose actual
	// SHA does not match the manifest entry. Get MUST reject.
	m := &Manifest{
		Version: 1,
		Images: []ImageEntry{{
			ID:     "x",
			URL:    srv.URL,
			SHA256: strings.Repeat("0", 64), // not the real SHA
			Size:   int64(len(body)),
			Format: "qcow2",
		}},
	}
	s := NewFilesystemStore(cacheRoot)

	_, err := s.Get(context.Background(), m, "x")
	if err == nil {
		t.Fatalf("expected SHA256 mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "SHA256") && !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error should mention SHA256: %v", err)
	}
	// Bad blob MUST be removed.
	badPath := filepath.Join(cacheRoot, "blobs", "sha256", strings.Repeat("0", 64))
	if _, statErr := os.Stat(badPath); statErr == nil {
		t.Fatalf("bad blob was NOT removed; expected the file at %s to be absent", badPath)
	}
}

func TestStore_Get_SizeMismatch_Rejects(t *testing.T) {
	body := []byte("real bytes")
	var hits int64
	srv := newCountingHTTPServer(body, &hits)
	defer srv.Close()

	m, _ := newSingleImageManifest(t, "x", srv.URL, body)
	m.Images[0].Size = int64(len(body) + 99) // wrong size
	s := NewFilesystemStore(t.TempDir())

	_, err := s.Get(context.Background(), m, "x")
	if err == nil {
		t.Fatalf("expected size mismatch error, got nil")
	}
}

func TestStore_Get_ConcurrentSerializesViaFlock(t *testing.T) {
	body := []byte("fake bytes")
	var hits int64
	srv := newCountingHTTPServer(body, &hits)
	defer srv.Close()

	m, _ := newSingleImageManifest(t, "x", srv.URL, body)
	s := NewFilesystemStore(t.TempDir())

	// 4 goroutines call Get concurrently; flock serializes; URL fetched once.
	var wg sync.WaitGroup
	const N = 4
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if _, err := s.Get(context.Background(), m, "x"); err != nil {
				t.Errorf("concurrent Get returned error: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("flock failed: expected 1 fetch, got %d", got)
	}
}
