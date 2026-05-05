package emulator

import (
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProcWalker struct {
	pids map[int]string
	err  error
}

func (f fakeProcWalker) PidComms() (map[int]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pids, nil
}

// PidCmdlines satisfies the extended procWalker interface (Group B).
// The pre-Group-B Cleanup() tests only exercise PidComms(); returning
// the same err here keeps error-propagation behaviour symmetric in case
// future callers route through PidCmdlines.
func (f fakeProcWalker) PidCmdlines() (map[int][]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

type fakeKiller struct {
	sent       map[int][]syscall.Signal
	aliveAfter map[syscall.Signal]map[int]bool
}

func newFakeKiller() *fakeKiller {
	return &fakeKiller{
		sent: map[int][]syscall.Signal{},
		aliveAfter: map[syscall.Signal]map[int]bool{
			syscall.SIGTERM: {},
		},
	}
}

func (f *fakeKiller) Signal(pid int, sig syscall.Signal) error {
	f.sent[pid] = append(f.sent[pid], sig)
	return nil
}

func (f *fakeKiller) Exists(pid int) bool {
	// Note: cleanupWithDeps calls Exists only during the SIGTERM
	// grace-window poll, never after SIGKILL — so the post-SIGKILL
	// branch from a prior implementation was dead code and has been
	// removed. The Surviving-after-SIGKILL path is controlled
	// instead by fakeKiller.Signal() return values; today Signal()
	// always returns nil so Surviving is structurally untestable
	// without further fake extension.
	sent := f.sent[pid]
	for _, s := range sent {
		if s == syscall.SIGKILL {
			return false
		}
		if s == syscall.SIGTERM {
			return f.aliveAfter[syscall.SIGTERM][pid]
		}
	}
	return true // never signalled — alive
}

// TestCleanup_NoMatches confirms an empty /proc state returns an empty
// report and sends no signals. Falsifiability: change the prefix
// matcher to "" → all PIDs would be Found and signalled. Test fails.
func TestCleanup_NoMatches(t *testing.T) {
	w := fakeProcWalker{pids: map[int]string{
		1234: "bash",
		5678: "node",
		9999: "java",
	}}
	k := newFakeKiller()

	report, err := cleanupWithDeps(context.Background(), w, k)
	require.NoError(t, err)
	assert.Empty(t, report.Found)
	assert.Empty(t, report.TerminatedTERM)
	assert.Empty(t, report.KilledKILL)
	assert.Empty(t, report.Surviving)
	assert.Empty(t, k.sent)
}

// TestCleanup_OneMatch_TerminatesOnSIGTERM confirms the happy path:
// one qemu-system PID is found, SIGTERM is sent, the PID exits within
// the grace window (fakeKiller.Exists returns false after SIGTERM by
// default), no SIGKILL needed.
func TestCleanup_OneMatch_TerminatesOnSIGTERM(t *testing.T) {
	w := fakeProcWalker{pids: map[int]string{
		1234: "bash",
		7777: "qemu-system-x86_64",
	}}
	k := newFakeKiller()

	report, err := cleanupWithDeps(context.Background(), w, k)
	require.NoError(t, err)
	assert.Equal(t, []int{7777}, report.Found)
	assert.Equal(t, []int{7777}, report.TerminatedTERM)
	assert.Empty(t, report.KilledKILL)
	assert.Equal(t, []syscall.Signal{syscall.SIGTERM}, k.sent[7777])
}

// TestCleanup_StrictPrefix is the falsifiability-rehearsal test for
// the prefix-matcher. Synthetic /proc contains "qemu-img" and "qemu"
// (NOT qemu-system processes). The strict prefix "qemu-system-" must
// NOT match them.
//
// Mutation: loosen prefix to "qemu-" → this test fails because PID
//
//	8888 (qemu-img) is now in Found.
//
// Reverted: yes.
func TestCleanup_StrictPrefix(t *testing.T) {
	w := fakeProcWalker{pids: map[int]string{
		7777: "qemu-system-x86_64", // legitimate match
		8888: "qemu-img",           // NOT a qemu-system process
		9999: "qemu",               // NOT a qemu-system process
	}}
	k := newFakeKiller()

	report, err := cleanupWithDeps(context.Background(), w, k)
	require.NoError(t, err)
	assert.Equal(t, []int{7777}, report.Found,
		"STRICT prefix qemu-system- MUST NOT match qemu-img or qemu")
	assert.Empty(t, k.sent[8888])
	assert.Empty(t, k.sent[9999])
}

// TestCleanup_PropagatesProcReadErr confirms procWalker errors surface
// to the caller (we don't silently swallow /proc read failures).
func TestCleanup_PropagatesProcReadErr(t *testing.T) {
	w := fakeProcWalker{err: errors.New("permission denied")}
	k := newFakeKiller()

	_, err := cleanupWithDeps(context.Background(), w, k)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

// TestCleanup_StragglerRequiresSIGKILL exercises the path where a
// qemu-system PID survives the SIGTERM grace window and requires
// SIGKILL. Without this test, a regression that removed the SIGKILL
// block would not be caught.
//
// Falsifiability: comment out the `for _, pid := range stragglers {
// k.Signal(pid, syscall.SIGKILL) ... }` block in cleanupWithDeps.
// Test fails: report.KilledKILL is empty when it should be []int{7777}.
// Reverted: yes.
func TestCleanup_StragglerRequiresSIGKILL(t *testing.T) {
	w := fakeProcWalker{pids: map[int]string{
		7777: "qemu-system-x86_64",
	}}
	k := newFakeKiller()
	k.aliveAfter[syscall.SIGTERM][7777] = true // survives SIGTERM grace window

	// Use a context that bounds the test runtime (the production
	// poll loop runs for up to 5 real seconds; we want the fake to
	// short-circuit faster). The fake's Exists() returns true for
	// 7777 throughout the SIGTERM-poll window, so the loop exhausts
	// its 5-second deadline naturally. Acceptable for a unit test.
	report, err := cleanupWithDeps(context.Background(), w, k)
	require.NoError(t, err)
	assert.Equal(t, []int{7777}, report.Found)
	assert.Empty(t, report.TerminatedTERM,
		"PID survived SIGTERM grace window, MUST NOT be in TerminatedTERM")
	assert.Equal(t, []int{7777}, report.KilledKILL,
		"PID surviving SIGTERM MUST be SIGKILLed and recorded in KilledKILL")
	assert.Empty(t, report.Surviving)
	// Verify both signals were sent in the right order
	assert.Equal(t, []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}, k.sent[7777],
		"SIGTERM MUST come before SIGKILL")
}

// ---------------------------------------------------------------------
// KillByPort tests — Group B clause 6.I extension
//
// Forensic anchor: the matrix-runner Teardown's `adb emu kill` retains
// its 30s grace, but stuck QEMU instances persist past it (clause
// 6.M-recorded behavior). Without a port-strict force-kill fast-path,
// the next iteration's Boot lands on 5556/5557 and the matrix
// accumulates concurrent emulators — observed flakes in the API 35 row
// of the 5-AVD matrix.
//
// SAFETY contract for KillByPort (the tests below verify each clause):
//   - Strict adjacent token match — substring `25554` must NOT match port 5554
//   - No-op on mismatch — concurrent emulators on other ports untouched
//   - SIGKILL only after 5s grace — graceful exit honored first
// ---------------------------------------------------------------------

type fakeProcWalkerWithCmdlines struct {
	cmdlines map[int][]string // pid → argv
}

func (f fakeProcWalkerWithCmdlines) PidComms() (map[int]string, error) {
	out := make(map[int]string)
	for pid, argv := range f.cmdlines {
		if len(argv) > 0 {
			out[pid] = argv[0]
		} else {
			out[pid] = ""
		}
	}
	return out, nil
}

func (f fakeProcWalkerWithCmdlines) PidCmdlines() (map[int][]string, error) {
	return f.cmdlines, nil
}

type fakeKillerByPort struct {
	signaled   map[int][]syscall.Signal
	aliveAfter map[syscall.Signal]map[int]bool // post-signal aliveness override
}

func newFakeKillerByPort() *fakeKillerByPort {
	return &fakeKillerByPort{
		signaled:   make(map[int][]syscall.Signal),
		aliveAfter: make(map[syscall.Signal]map[int]bool),
	}
}

func (f *fakeKillerByPort) Signal(pid int, sig syscall.Signal) error {
	f.signaled[pid] = append(f.signaled[pid], sig)
	return nil
}

func (f *fakeKillerByPort) Exists(pid int) bool {
	// Default: SIGTERM clears the process; SIGKILL clears it.
	// Tests override aliveAfter[sig][pid] = true to keep the process
	// alive past the corresponding signal (forces SIGKILL path).
	signals, ok := f.signaled[pid]
	if !ok {
		return true // never signaled → still alive
	}
	last := signals[len(signals)-1]
	if alive, override := f.aliveAfter[last][pid]; override {
		return alive
	}
	return false
}

func TestKillByPort_NoMatch_NoOp(t *testing.T) {
	w := fakeProcWalkerWithCmdlines{cmdlines: map[int][]string{
		1234: {"qemu-system-x86_64", "-avd", "Pixel_9a", "-port", "5556"},
		5678: {"chrome", "--user-data-dir=/tmp/x"},
	}}
	k := newFakeKillerByPort()
	report, err := killByPortWithDeps(context.Background(), 5554, w, k)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Matched != 0 {
		t.Fatalf("expected Matched=0 (no proc has -port 5554 adjacent), got %d", report.Matched)
	}
	if len(k.signaled) != 0 {
		t.Fatalf("expected no kill signals issued, got %v", k.signaled)
	}
}

func TestKillByPort_StrictAdjacentMatch(t *testing.T) {
	w := fakeProcWalkerWithCmdlines{cmdlines: map[int][]string{
		1111: {"qemu-system-x86_64", "-avd", "A1", "-port", "5554"}, // MATCH
		2222: {"qemu-system-x86_64", "-avd", "A2", "-port", "5556"}, // no match
	}}
	k := newFakeKillerByPort()
	report, err := killByPortWithDeps(context.Background(), 5554, w, k)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Matched != 1 {
		t.Fatalf("expected Matched=1, got %d", report.Matched)
	}
	if len(report.Sigtermed) != 1 || report.Sigtermed[0] != 1111 {
		t.Fatalf("expected Sigtermed=[1111], got %v", report.Sigtermed)
	}
	if _, signaled := k.signaled[2222]; signaled {
		t.Fatalf("pid 2222 was signaled despite different port — safety violation")
	}
}

func TestKillByPort_SubstringSafety(t *testing.T) {
	// pid 9999 has the literal string "5554" inside its argv but NOT
	// adjacent to "-port". KillByPort(5554) MUST NOT match it.
	w := fakeProcWalkerWithCmdlines{cmdlines: map[int][]string{
		9999: {"qemu-system-x86_64", "-avd", "A1", "-port", "25554"}, // 25554 ≠ 5554
		8888: {"qemu-system-x86_64", "-avd", "A2", "-pidfile", "/tmp/5554.pid"},
	}}
	k := newFakeKillerByPort()
	report, err := killByPortWithDeps(context.Background(), 5554, w, k)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Matched != 0 {
		t.Fatalf("expected Matched=0 (no adjacent token pair), got %d (signaled=%v)",
			report.Matched, k.signaled)
	}
}

func TestKillByPort_RequiresSIGKILL_AfterGrace(t *testing.T) {
	w := fakeProcWalkerWithCmdlines{cmdlines: map[int][]string{
		7777: {"qemu-system-x86_64", "-avd", "A1", "-port", "5554"},
	}}
	k := newFakeKillerByPort()
	// Make pid 7777 survive SIGTERM — forces the SIGKILL grace path.
	// After SIGKILL, default Exists() returns false, so the process
	// is reported as Sigkilled, not Surviving.
	k.aliveAfter[syscall.SIGTERM] = map[int]bool{7777: true}
	report, err := killByPortWithDeps(context.Background(), 5554, w, k)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Matched != 1 {
		t.Fatalf("expected Matched=1, got %d", report.Matched)
	}
	if len(report.Sigkilled) != 1 || report.Sigkilled[0] != 7777 {
		t.Fatalf("expected Sigkilled=[7777], got %v", report.Sigkilled)
	}
}
