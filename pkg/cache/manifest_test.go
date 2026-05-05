package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vm-images.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestLoadManifest_HappyPath(t *testing.T) {
	path := writeFixture(t, `{
	  "version": 1,
	  "images": [
	    {"id":"alpine-3.20-x86_64","url":"https://example.com/a.qcow2","sha256":"`+strings.Repeat("a", 64)+`","size":12345,"format":"qcow2"}
	  ]
	}`)
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Images) != 1 || m.Images[0].ID != "alpine-3.20-x86_64" {
		t.Fatalf("got %+v", m)
	}
}

func TestLoadManifest_RejectsDuplicateID(t *testing.T) {
	path := writeFixture(t, `{
	  "version":1,
	  "images":[
	    {"id":"a","url":"https://e.com/1","sha256":"`+strings.Repeat("a", 64)+`","size":1,"format":"qcow2"},
	    {"id":"a","url":"https://e.com/2","sha256":"`+strings.Repeat("b", 64)+`","size":2,"format":"qcow2"}
	  ]
	}`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatalf("expected duplicate-ID error, got nil")
	}
}

func TestLoadManifest_RejectsMalformedSHA256(t *testing.T) {
	// 63 chars (one short) — invalid hex-encoded SHA-256
	path := writeFixture(t, `{
	  "version":1,
	  "images":[
	    {"id":"a","url":"https://e.com/1","sha256":"`+strings.Repeat("a", 63)+`","size":1,"format":"qcow2"}
	  ]
	}`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatalf("expected malformed-SHA256 error, got nil")
	}
}

func TestLoadManifest_RejectsSchemaVersionMismatch(t *testing.T) {
	path := writeFixture(t, `{"version":99,"images":[]}`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatalf("expected schema-version error, got nil")
	}
}

func TestLoadManifest_FindByID(t *testing.T) {
	path := writeFixture(t, `{
	  "version":1,
	  "images":[
	    {"id":"a","url":"https://e.com/1","sha256":"`+strings.Repeat("a", 64)+`","size":1,"format":"qcow2"}
	  ]
	}`)
	m, _ := LoadManifest(path)
	got, err := m.FindByID("a")
	if err != nil || got.ID != "a" {
		t.Fatalf("FindByID(a): got=%+v err=%v", got, err)
	}
	if _, err := m.FindByID("nope"); err == nil {
		t.Fatalf("FindByID(nope): expected error, got nil")
	}
}
