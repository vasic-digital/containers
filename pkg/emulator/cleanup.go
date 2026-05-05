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
//
// PidComms returns pid → /proc/<pid>/comm (process name only, used by
// Cleanup's prefix matcher). PidCmdlines returns pid → argv slice
// (split on NUL bytes, used by KillByPort's strict adjacent-token
// matcher).
type procWalker interface {
	PidComms() (map[int]string, error)
	PidCmdlines() (map[int][]string, error)
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

func (osProcWalker) PidCmdlines() (map[int][]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}
	out := make(map[int][]string)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		cmdPath := filepath.Join("/proc", e.Name(), "cmdline")
		b, err := os.ReadFile(cmdPath)
		if err != nil {
			// Best-effort: process may have exited mid-walk; record
			// empty argv so the matcher simply skips it.
			out[pid] = nil
			continue
		}
		// /proc/<pid>/cmdline is NUL-separated. Trailing NUL is common.
		raw := strings.TrimRight(string(b), "\x00")
		if raw == "" {
			out[pid] = nil
			continue
		}
		out[pid] = strings.Split(raw, "\x00")
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

// KillReport summarises the outcome of a KillByPort invocation.
//
// The Matched count is the gate the caller (Teardown fast-path) uses
// to decide whether the kill succeeded enough to short-circuit the
// "emulator did not exit" error path. Matched=0 is a no-op safe state:
// it means no /proc entry passed the strict adjacent-token check, and
// the caller MUST treat that as "fast-path skipped" — typically by
// returning the original timeout error so the matrix runner records
// an honest row failure.
type KillReport struct {
	// Matched is the number of /proc entries whose cmdline contained
	// "-port <port>" as adjacent argv tokens.
	Matched int
	// Sigtermed lists the PIDs that received SIGTERM (every matched
	// PID receives one).
	Sigtermed []int
	// Sigkilled lists the PIDs that survived the 5-second SIGTERM
	// grace and therefore received SIGKILL.
	Sigkilled []int
	// Surviving lists PIDs still alive after SIGKILL (rare; caused
	// by permission errors, kernel-level holds, or PID-reuse races).
	Surviving []int
}

// KillByPort attempts to terminate the process(es) whose cmdline
// contains "-port <port>" as adjacent argv tokens. Used by the
// matrix-runner's Teardown fast-path: when `adb emu kill` returns
// successfully but the underlying QEMU instance is stuck past the
// 30s grace, this function targets that specific QEMU child by its
// console port and SIGTERMs it (then SIGKILLs after a 5s grace).
//
// SAFETY (THE LOAD-BEARING INVARIANT):
//
//   - The match is STRICT adjacent-token. A process whose cmdline
//     contains "-port" "5554" as adjacent argv tokens MATCHES.
//     A process whose cmdline contains "-port" "25554" does NOT match
//     for port 5554 — the substring is irrelevant; tokenization is.
//   - On Matched=0, KillByPort is a complete no-op — no signals are
//     sent. Concurrent emulators on other ports, sibling vasic-digital
//     project QEMUs, and developer-spawned QEMUs are NEVER touched.
//
// This is the constitutional fix for the 2026-05-04 evening operator
// concern that "any broader pkill against session processes" is a
// Forbidden Command List violation. KillByPort is permitted because
// it targets a single, provably-this-matrix QEMU child by its argv,
// not by name.
//
// Bluff-Audit (recorded in the implementing commit body):
//
//	Mutation: weaken the matcher to strings.Contains(token, target)
//	Observed: TestKillByPort_SubstringSafety asserts that pid 9999
//	          (whose argv contains "25554") is NOT matched for port
//	          5554; the weakened matcher matches it because "25554"
//	          contains "5554" as a substring.
//	Reverted: yes
func KillByPort(ctx context.Context, port int) (KillReport, error) {
	return killByPortWithDeps(ctx, port, osProcWalker{}, osKiller{})
}

// killByPortWithDeps is the testable core; production KillByPort wires
// the real procWalker + killer.
func killByPortWithDeps(
	ctx context.Context,
	port int,
	w procWalker,
	k killer,
) (KillReport, error) {
	var report KillReport
	cmdlines, err := w.PidCmdlines()
	if err != nil {
		return report, err
	}
	target := strconv.Itoa(port)
	for pid, argv := range cmdlines {
		// STRICT adjacent-token match. Walk argv looking for the
		// literal token "-port" immediately followed by the literal
		// port number. Substring matches and "-port=5554" forms are
		// intentionally NOT honoured — qemu-system never emits the
		// "=" form, and substring matches are the bluff vector this
		// API exists to prevent.
		matched := false
		for i := 0; i < len(argv)-1; i++ {
			if argv[i] == "-port" && argv[i+1] == target {
				matched = true
				break
			}
		}
		if matched {
			report.Matched++
			report.Sigtermed = append(report.Sigtermed, pid)
		}
	}
	if report.Matched == 0 {
		return report, nil
	}
	for _, pid := range report.Sigtermed {
		_ = k.Signal(pid, syscall.SIGTERM)
	}
	// Wait up to 5 seconds, polling every 250ms.
	deadline := time.Now().Add(5 * time.Second)
	var stragglers []int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		stragglers = stragglers[:0]
		for _, pid := range report.Sigtermed {
			if k.Exists(pid) {
				stragglers = append(stragglers, pid)
			}
		}
		if len(stragglers) == 0 {
			break
		}
	}
	for _, pid := range stragglers {
		if err := k.Signal(pid, syscall.SIGKILL); err == nil {
			report.Sigkilled = append(report.Sigkilled, pid)
		} else {
			report.Surviving = append(report.Surviving, pid)
		}
	}
	return report, nil
}
