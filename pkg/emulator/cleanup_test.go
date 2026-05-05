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

type fakeKiller struct {
	sent       map[int][]syscall.Signal
	aliveAfter map[syscall.Signal]map[int]bool
}

func newFakeKiller() *fakeKiller {
	return &fakeKiller{
		sent: map[int][]syscall.Signal{},
		aliveAfter: map[syscall.Signal]map[int]bool{
			syscall.SIGTERM: {},
			syscall.SIGKILL: {},
		},
	}
}

func (f *fakeKiller) Signal(pid int, sig syscall.Signal) error {
	f.sent[pid] = append(f.sent[pid], sig)
	return nil
}

func (f *fakeKiller) Exists(pid int) bool {
	if _, killed := f.aliveAfter[syscall.SIGKILL][pid]; killed {
		return true
	}
	sent := f.sent[pid]
	for _, s := range sent {
		if s == syscall.SIGKILL {
			return false
		}
		if s == syscall.SIGTERM {
			return f.aliveAfter[syscall.SIGTERM][pid]
		}
	}
	return true
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
