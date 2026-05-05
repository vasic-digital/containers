package emulator

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaptureScreenshot_WritesPNGBytes feeds canned PNG bytes through
// the fakeExecutor and verifies CaptureScreenshot writes them verbatim
// to the destination path.
//
// The test asserts on user-visible state (the on-disk file contents)
// per Sixth Law clause 3 — not on call-count. Mock-was-invoked-N-times
// is a permitted secondary check; the primary assertion is byte-equality
// of the produced file.
func TestCaptureScreenshot_WritesPNGBytes(t *testing.T) {
	// Minimal valid PNG bytes (8-byte signature + IHDR-stub).
	// Real adb output is a full PNG; we use a recognisable prefix so
	// the assertion is unambiguous.
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 'I', 'H', 'D', 'R'}
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{
			"/usr/local/bin/adb -s localhost:5554 exec-out screencap -p": {
				Out: pngBytes,
			},
		},
	}
	dst := filepath.Join(t.TempDir(), "subdir", "screenshot.png")
	err := CaptureScreenshot(context.Background(), exec, "/usr/local/bin/adb", "emulator-5554", dst)
	if err != nil {
		t.Fatalf("CaptureScreenshot: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read written screenshot: %v", err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Fatalf("written bytes diverge from adb output; want %x got %x", pngBytes, got)
	}
}

// TestCaptureScreenshot_AdbErrorPropagated asserts adb failures surface
// as errors (so the matrix runner can log them; does NOT flip the row
// — that contract lives in matrix.go).
func TestCaptureScreenshot_AdbErrorPropagated(t *testing.T) {
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{
			"/usr/local/bin/adb -s localhost:5554 exec-out screencap -p": {
				Err: errOpaque("device disconnected"),
			},
		},
	}
	dst := filepath.Join(t.TempDir(), "screenshot.png")
	err := CaptureScreenshot(context.Background(), exec, "/usr/local/bin/adb", "emulator-5554", dst)
	if err == nil {
		t.Fatalf("expected error when adb fails, got nil")
	}
	if !strings.Contains(err.Error(), "adb screencap") {
		t.Fatalf("error should carry the 'adb screencap' prefix; got %v", err)
	}
	// File MUST NOT be created on adb failure.
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Fatalf("destination file should NOT exist after adb failure; got: %s", dst)
	}
}

// TestCaptureScreenshot_EmptyBytesIsError asserts an unexpected empty
// adb output produces an honest error rather than a 0-byte file.
func TestCaptureScreenshot_EmptyBytesIsError(t *testing.T) {
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{
			"/usr/local/bin/adb -s localhost:5554 exec-out screencap -p": {Out: []byte{}},
		},
	}
	dst := filepath.Join(t.TempDir(), "screenshot.png")
	err := CaptureScreenshot(context.Background(), exec, "/usr/local/bin/adb", "emulator-5554", dst)
	if err == nil {
		t.Fatalf("expected error when adb returns empty bytes, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error should mention the empty-bytes failure; got %v", err)
	}
}
