package emulator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// CaptureScreenshot grabs a screenshot from the running emulator and
// writes the PNG bytes to dstPath. Uses `adb -s <serial> exec-out
// screencap -p` which streams PNG-encoded bytes on stdout (no temp file
// on the device required).
//
// Anti-bluff posture (clauses 6.J/6.L): screenshot capture is forensic
// observability, NOT a gate. A failure here MUST NOT flip a row's
// Passed signal — the row already failed; missing the screenshot just
// means we have less to work with post-hoc. Callers therefore log the
// error and continue; they do NOT propagate it as a row-fatal error.
//
// We DO return the error so the caller can choose to log + continue;
// silently swallowing the error inside this function would be a clause
// 6.J bluff vector by hiding actionable signal.
func CaptureScreenshot(
	ctx context.Context,
	exec CommandExecutor,
	adbBinary, serial, dstPath string,
) error {
	target := fmt.Sprintf("localhost:%d", parseSerialPort(serial))
	out, err := exec.Execute(ctx, adbBinary, "-s", target, "exec-out", "screencap", "-p")
	if err != nil {
		return fmt.Errorf("adb screencap: %w", err)
	}
	if len(out) == 0 {
		return fmt.Errorf("adb screencap returned empty bytes")
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for screenshot: %w", err)
	}
	if err := os.WriteFile(dstPath, out, 0o644); err != nil {
		return fmt.Errorf("write screenshot %s: %w", dstPath, err)
	}
	return nil
}
