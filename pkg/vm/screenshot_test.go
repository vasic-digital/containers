package vm

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaptureScreenshotVM_WritesPPMBytes drives CaptureScreenshotVM
// through the fake QMP client and asserts the resulting host file
// contains the bytes the fake "wrote" via screendumpFile.
func TestCaptureScreenshotVM_WritesPPMBytes(t *testing.T) {
	// Minimal PPM-ish payload — real qemu writes a P6 PPM.
	ppmBytes := []byte("P6\n2 2\n255\n\xff\x00\x00\x00\xff\x00\x00\x00\xff\xff\xff\xff")
	qmp := &fakeQMPClient{screendumpFile: ppmBytes}
	dst := filepath.Join(t.TempDir(), "subdir", "screenshot.ppm")

	err := CaptureScreenshotVM(context.Background(), qmp, 14444, dst)
	if err != nil {
		t.Fatalf("CaptureScreenshotVM: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read written ppm: %v", err)
	}
	if !bytes.Equal(got, ppmBytes) {
		t.Fatalf("written bytes diverge from canned screendump output")
	}
	if len(qmp.screendumpPaths) != 1 || qmp.screendumpPaths[0] != dst {
		t.Fatalf("expected exactly 1 screendump call to %q; got %v", dst, qmp.screendumpPaths)
	}
}

// TestCaptureScreenshotVM_QMPDialError_Propagated asserts a Dial failure
// surfaces upward.
func TestCaptureScreenshotVM_QMPDialError_Propagated(t *testing.T) {
	qmp := &fakeQMPClient{dialError: errors.New("connection refused")}
	dst := filepath.Join(t.TempDir(), "screenshot.ppm")
	err := CaptureScreenshotVM(context.Background(), qmp, 14444, dst)
	if err == nil {
		t.Fatalf("expected error on qmp dial failure")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error should cite the dial failure; got %v", err)
	}
	// File MUST NOT be created on dial failure.
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Fatalf("destination file should NOT exist after qmp dial failure")
	}
}

// TestCaptureScreenshotVM_ScreendumpError_Propagated asserts a
// screendump-side failure surfaces upward.
func TestCaptureScreenshotVM_ScreendumpError_Propagated(t *testing.T) {
	qmp := &fakeQMPClient{screendumpErr: errors.New("permission denied")}
	dst := filepath.Join(t.TempDir(), "screenshot.ppm")
	err := CaptureScreenshotVM(context.Background(), qmp, 14444, dst)
	if err == nil {
		t.Fatalf("expected error on screendump failure")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error should cite screendump failure; got %v", err)
	}
}
