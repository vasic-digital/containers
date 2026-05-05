package emulator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CleanupReport summarises the outcome of a Cleanup invocation.
type CleanupReport struct {
	Found          []int // PIDs whose /proc/<pid>/comm matched "qemu-system-*"
	TerminatedTERM []int // PIDs that exited within the SIGTERM grace window
	KilledKILL     []int // PIDs that required SIGKILL
	Surviving      []int // PIDs still alive after SIGKILL (rare; permission errors)
	SkippedReadErr []int // PIDs whose /proc/<pid>/comm couldn't be read (permission/race)
}

// procWalker abstracts /proc enumeration so cleanup_test.go can inject
// synthetic /proc data. Production walks the real /proc.
type procWalker interface {
	PidComms() (map[int]string, error)
}

type osProcWalker struct{}

func (osProcWalker) PidComms() (map[int]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}
	out := make(map[int]string)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		commPath := filepath.Join("/proc", e.Name(), "comm")
		b, err := os.ReadFile(commPath)
		if err != nil {
			// Best-effort: process may have exited mid-walk
			out[pid] = ""
			continue
		}
		out[pid] = strings.TrimSpace(string(b))
	}
	return out, nil
}

// killer abstracts signalling for testability.
type killer interface {
	Signal(pid int, sig syscall.Signal) error
	Exists(pid int) bool
}

type osKiller struct{}

func (osKiller) Signal(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func (osKiller) Exists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// Cleanup walks /proc, finds processes whose comm has the prefix
// "qemu-system-", sends SIGTERM, waits up to 5 seconds for graceful
// exit, then SIGKILLs stragglers. Returns a CleanupReport.
//
// This API replaces the per-script ad-hoc `pkill qemu-system`
// invocations that the Forbidden Command List would otherwise reject —
// `pkill` against session processes is forbidden, but a typed
// in-package cleanup that targets a strict process-name allowlist
// (exact prefix "qemu-system-" with trailing dash) is permitted per
// Containers' STRONGER §6.M variant.
//
// Bluff-Audit (recorded in the implementing commit body):
//
//	Mutation: loosen the prefix matcher from "qemu-system-" to "qemu-"
//	Observed: TestCleanup_StrictPrefix asserts that a synthetic
//	          /proc fixture containing "qemu-img" is NOT collected;
//	          the loosened matcher would include it, failing the test.
//	Reverted: yes
func Cleanup(ctx context.Context) (CleanupReport, error) {
	return cleanupWithDeps(ctx, osProcWalker{}, osKiller{})
}

// cleanupWithDeps is the testable core. Production uses Cleanup; tests
// inject synthetic procWalker + killer.
func cleanupWithDeps(ctx context.Context, w procWalker, k killer) (CleanupReport, error) {
	var report CleanupReport
	pidComms, err := w.PidComms()
	if err != nil {
		return report, err
	}
	for pid, comm := range pidComms {
		if comm == "" {
			report.SkippedReadErr = append(report.SkippedReadErr, pid)
			continue
		}
		// STRICT prefix: "qemu-system-" with trailing dash. NOT "qemu-".
		// Falsifiability mutation target — see TestCleanup_StrictPrefix.
		if strings.HasPrefix(comm, "qemu-system-") {
			report.Found = append(report.Found, pid)
		}
	}
	if len(report.Found) == 0 {
		return report, nil
	}
	// Send SIGTERM to all found PIDs
	for _, pid := range report.Found {
		_ = k.Signal(pid, syscall.SIGTERM)
	}
	// Wait up to 5 seconds, polling every 250ms
	deadline := time.Now().Add(5 * time.Second)
	var stragglers []int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		stragglers = stragglers[:0]
		for _, pid := range report.Found {
			if k.Exists(pid) {
				stragglers = append(stragglers, pid)
			}
		}
		if len(stragglers) == 0 {
			break
		}
	}
	// Mark PIDs that exited within grace window as TerminatedTERM
	terminated := make(map[int]bool)
	for _, pid := range report.Found {
		terminated[pid] = true
	}
	for _, pid := range stragglers {
		terminated[pid] = false
	}
	for _, pid := range report.Found {
		if terminated[pid] {
			report.TerminatedTERM = append(report.TerminatedTERM, pid)
		}
	}
	// SIGKILL stragglers
	for _, pid := range stragglers {
		if err := k.Signal(pid, syscall.SIGKILL); err == nil {
			report.KilledKILL = append(report.KilledKILL, pid)
		} else {
			report.Surviving = append(report.Surviving, pid)
		}
	}
	return report, nil
}
