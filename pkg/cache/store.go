package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Store is the cache contract.
//
// Get returns the local path of the image's bytes. On cache miss, the
// image is fetched from the manifest's URL, SHA-256 verified, then
// written atomically. Verify recomputes the SHA and compares against
// the manifest. Refresh removes the cached blob and fetches it again.
type Store interface {
	Get(ctx context.Context, m *Manifest, imageID string) (path string, err error)
	Verify(ctx context.Context, m *Manifest, imageID string) error
	Refresh(ctx context.Context, m *Manifest, imageID string) error
}

// FilesystemStore is the default Store impl. Cache root layout:
//
//	<root>/blobs/sha256/<full-hash>     image bytes
//	<root>/lockfiles/<sha-prefix>.lock  flock per-image (concurrent-fetch safety)
type FilesystemStore struct {
	root string
	// httpClient injection seam for tests; production uses http.DefaultClient.
	httpClient *http.Client
	// in-process mutex map: serializes Get for the same imageID across
	// goroutines in the same process. Cross-process safety comes from
	// the flock on disk; in-process callers don't need to acquire flock
	// twice.
	mu     sync.Mutex
	keymus map[string]*sync.Mutex
}

// NewFilesystemStore constructs a Store rooted at cacheRoot.
// $XDG_CACHE_HOME/vasic-digital/containers-images/ is the production path;
// tests pass t.TempDir() to isolate.
func NewFilesystemStore(cacheRoot string) *FilesystemStore {
	return &FilesystemStore{
		root:       cacheRoot,
		httpClient: http.DefaultClient,
		keymus:     make(map[string]*sync.Mutex),
	}
}

func (s *FilesystemStore) blobsDir() string     { return filepath.Join(s.root, "blobs", "sha256") }
func (s *FilesystemStore) lockfilesDir() string { return filepath.Join(s.root, "lockfiles") }

func (s *FilesystemStore) blobPath(sha string) string {
	return filepath.Join(s.blobsDir(), sha)
}

func (s *FilesystemStore) lockPath(sha string) string {
	prefix := sha
	if len(sha) > 8 {
		prefix = sha[:8]
	}
	return filepath.Join(s.lockfilesDir(), prefix+".lock")
}

func (s *FilesystemStore) keymu(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	mu, ok := s.keymus[id]
	if !ok {
		mu = &sync.Mutex{}
		s.keymus[id] = mu
	}
	return mu
}

// Get returns the local path of the image's bytes. On cache miss, the
// image is fetched from URL, SHA-256 verified, then written atomically
// to the blob path. SHA mismatch removes the partial file and returns
// an error.
//
// Concurrency: in-process callers serialize via a per-id sync.Mutex;
// cross-process callers serialize via a syscall.Flock on a sidecar
// lock file. Both paths re-check the cache fast-path after acquiring
// the lock so the second waiter discovers the first waiter's blob and
// skips the fetch.
func (s *FilesystemStore) Get(ctx context.Context, m *Manifest, imageID string) (string, error) {
	entry, err := m.FindByID(imageID)
	if err != nil {
		return "", err
	}
	final := s.blobPath(entry.SHA256)

	// Fast path — already cached.
	if _, err := os.Stat(final); err == nil {
		return final, nil
	}

	// In-process serialization first.
	mu := s.keymu(imageID)
	mu.Lock()
	defer mu.Unlock()

	// After winning the in-process lock, re-check the fast path.
	if _, err := os.Stat(final); err == nil {
		return final, nil
	}

	// Cross-process serialization via flock.
	if err := os.MkdirAll(s.blobsDir(), 0o755); err != nil {
		return "", fmt.Errorf("mkdir blobs: %w", err)
	}
	if err := os.MkdirAll(s.lockfilesDir(), 0o755); err != nil {
		return "", fmt.Errorf("mkdir lockfiles: %w", err)
	}
	lockPath := s.lockPath(entry.SHA256)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", fmt.Errorf("open lock %s: %w", lockPath, err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("flock %s: %w", lockPath, err)
	}
	defer func() { _ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN) }()

	// Re-check fast path AFTER acquiring flock — another process may
	// have populated it while we waited.
	if _, err := os.Stat(final); err == nil {
		return final, nil
	}

	// Fetch.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, entry.URL, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", entry.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: HTTP %d", entry.URL, resp.StatusCode)
	}

	tmp, err := os.CreateTemp(s.blobsDir(), "incoming-*")
	if err != nil {
		return "", fmt.Errorf("create tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), resp.Body)
	if cerr := tmp.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write tempfile: %w", err)
	}

	gotSHA := hex.EncodeToString(hasher.Sum(nil))
	if gotSHA != entry.SHA256 {
		_ = os.Remove(tmpPath)
		// Also remove the final-path blob if a partial one ever appeared
		// at it (defensive — shouldn't normally exist).
		_ = os.Remove(final)
		return "", fmt.Errorf("image %q: SHA256 mismatch (got %s, want %s)",
			entry.ID, gotSHA, entry.SHA256)
	}
	if entry.Size != 0 && written != entry.Size {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("image %q: size mismatch (got %d, want %d)",
			entry.ID, written, entry.Size)
	}

	if err := os.Rename(tmpPath, final); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("atomic rename %s → %s: %w", tmpPath, final, err)
	}
	return final, nil
}

// Verify recomputes the SHA-256 of the cached blob and compares to the
// manifest. Returns nil iff the blob exists AND its bytes hash to the
// declared SHA. Used by tooling that audits cache integrity.
func (s *FilesystemStore) Verify(ctx context.Context, m *Manifest, imageID string) error {
	entry, err := m.FindByID(imageID)
	if err != nil {
		return err
	}
	path := s.blobPath(entry.SHA256)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verify %q: %w", entry.ID, err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("verify %q: %w", entry.ID, err)
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if got != entry.SHA256 {
		return fmt.Errorf("verify %q: SHA256 drift (got %s, want %s)",
			entry.ID, got, entry.SHA256)
	}
	return nil
}

// Refresh removes the cached blob and fetches it again. Used by tooling
// when the operator deliberately bumps a manifest entry's SHA.
func (s *FilesystemStore) Refresh(ctx context.Context, m *Manifest, imageID string) error {
	entry, err := m.FindByID(imageID)
	if err != nil {
		return err
	}
	_ = os.Remove(s.blobPath(entry.SHA256))
	_, err = s.Get(ctx, m, imageID)
	return err
}
