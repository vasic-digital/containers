package vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CaptureScreenshotVM captures the guest's framebuffer to dstPath using
// QMP's `screendump` command. The output format is PPM (per QEMU's
// documentation); callers MAY post-process to PNG if desired.
//
// IMPORTANT — host path semantics: QEMU's `screendump` writes the PPM
// to a path INTERPRETED ON THE HOST (the QEMU binary runs on the host;
// QMP is its monitor socket). There is therefore NO guest→host SCP
// step: the file lands on the host directly at the path we send.
//
// The function is best-effort observability per Sixth Law clause 3:
// failures here are logged by the matrix runner but do NOT flip the
// row's Passed signal.
//
// QMP protocol used (line-delimited JSON):
//
//	→ {"execute":"screendump","arguments":{"filename":"<dstPath>"}}
//	← {"return":{}}                      // OK
//	← {"error":{"class":"...", ...}}     // Failure
//
// We do NOT wait for a guarantee the file is fully written before
// returning — QEMU writes to the file synchronously in the screendump
// handler. After Close() the file is on disk; the caller can stat it.
func CaptureScreenshotVM(
	ctx context.Context,
	qmp qmpClient,
	monitorPort int,
	dstPath string,
) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for screenshot: %w", err)
	}
	// Open a fresh QMP connection — we cannot reuse the matrix runner's
	// Teardown-time QMP because that one is closed after powerdown.
	// Dial returns when capabilities are negotiated; subsequent
	// Screendump issues the JSON command.
	if err := qmp.Dial(ctx, monitorPort, 5*time.Second); err != nil {
		return fmt.Errorf("qmp dial for screendump: %w", err)
	}
	defer func() { _ = qmp.Close() }()
	if err := qmp.Screendump(ctx, dstPath); err != nil {
		return fmt.Errorf("qmp screendump: %w", err)
	}
	// Sanity-check the file exists. If QEMU rejected the path silently
	// (filesystem permission, dir not present), the file won't be
	// there and we should surface that.
	if _, err := os.Stat(dstPath); err != nil {
		return fmt.Errorf("screendump produced no file at %s: %w", dstPath, err)
	}
	return nil
}
