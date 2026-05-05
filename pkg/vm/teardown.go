package vm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"digital.vasic.containers/pkg/emulator"
)

// killByPortHook is the package-level seam tests use to substitute a
// fake KillByPort implementation. Production uses pkg/emulator's
// KillByPort directly (already strict-adjacent, already
// constitutionally vetted in Group B Phase A).
//
// NOTE: tests that override this MUST NOT use t.Parallel(). The
// swap-and-restore pattern (`prev := X; X = ...; defer func() { X = prev }()`)
// is not safe against concurrent test functions racing on the
// package-level var. (Same convention as pkg/emulator/android.go.)
var killByPortHook = emulator.KillByPort

// teardownGracePeriod is the wall-clock time Teardown waits between
// initiating QMP graceful shutdown and falling through to the
// KillByPort fast-path. Production: 30 seconds. Tests override.
var teardownGracePeriod = 30 * time.Second

// Teardown attempts a 3-stage shutdown:
//
//  1. QMP system_powerdown (initiates ACPI shutdown in the guest)
//  2. wait teardownGracePeriod for QEMU process to exit
//  3. KillByPort fast-path on the monitor port — strict-adjacent
//     argv match. Skip-on-mismatch (Matched==0) returns the original
//     "did not exit" error. Group B Teardown pattern.
//
// The skip-on-mismatch behavior is the load-bearing safety property
// the falsifiability rehearsal targets — see TestTeardown_FastPath_-
// SkipsOnMismatch. A weakened Teardown that returns nil on Matched=0
// would silently lie about successful teardown of a stuck VM and
// corrupt the matrix runner's row outcome.
func (v *QEMUVM) Teardown(ctx context.Context, monitorPort, sshPort int) error {
	// Stage 1: QMP powerdown — best-effort.
	if v.qmp != nil {
		if err := v.qmp.Dial(ctx, monitorPort, 5*time.Second); err == nil {
			_ = v.qmp.SystemPowerdown(ctx)
			_ = v.qmp.Close()
		}
	}

	// Stage 2: wait for graceful exit. We can't directly observe the
	// QEMU process from here without a process handle; we sleep.
	// Production gets a richer signal from QMP's SHUTDOWN event;
	// implementation TBD when the real QMP client lands. For now,
	// the unit-test path uses teardownGracePeriod=200ms and the
	// production path uses 30s; the sleep is honest about that.
	deadline := time.Now().Add(teardownGracePeriod)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Stage 3: KillByPort fast-path on the monitor port.
	report, kerr := killByPortHook(ctx, monitorPort)
	if kerr != nil {
		// Forensic-only — log via fmt.Errorf wrap; do not block return.
		_ = kerr
	}
	if report.Matched == 0 {
		return fmt.Errorf(
			"vm on monitor port %d did not exit within %s; KillByPort matched 0 processes (skip-on-mismatch safety)",
			monitorPort, teardownGracePeriod,
		)
	}
	if len(report.Surviving) > 0 {
		return errors.New("vm Teardown: KillByPort succeeded but some PIDs surviving SIGKILL")
	}
	return nil
}
