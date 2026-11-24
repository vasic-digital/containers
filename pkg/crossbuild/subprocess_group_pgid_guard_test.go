//go:build unix

package crossbuild

import (
	"errors"
	"syscall"
	"testing"
)

// Constitution §11.4.263: a process-group signal must never be sent with a
// pgid <= 1. kill(-1, sig) signals every process the caller may signal, and
// kill(0, sig) signals the caller's own group. These tests pin that
// killProcessGroup refuses such a pid WITHOUT calling the kill function,
// and that a valid pid is signalled as its negative (the group).

func TestKillProcessGroup_RefusesPidAtOrBelowOne(t *testing.T) {
	for _, pid := range []int{1, 0, -1, -4242} {
		called := false
		kill := func(int, syscall.Signal) error { called = true; return nil }
		err := killProcessGroup(pid, syscall.SIGKILL, kill)
		if err == nil {
			t.Fatalf("pid %d: expected a refusal error, got nil", pid)
		}
		if !errors.Is(err, errUnsafeProcessGroup) {
			t.Fatalf("pid %d: expected errUnsafeProcessGroup, got %v", pid, err)
		}
		if called {
			t.Fatalf("pid %d: kill function was called; a group signal with pgid <= 1 must never be sent", pid)
		}
	}
}

func TestKillProcessGroup_SignalsNegativePidForValidPid(t *testing.T) {
	for _, pid := range []int{2, 4242} {
		gotPid, gotSig, calls := 0, syscall.Signal(0), 0
		kill := func(p int, s syscall.Signal) error { gotPid, gotSig, calls = p, s, calls+1; return nil }
		if err := killProcessGroup(pid, syscall.SIGKILL, kill); err != nil {
			t.Fatalf("pid %d: unexpected error %v", pid, err)
		}
		if calls != 1 || gotPid != -pid || gotSig != syscall.SIGKILL {
			t.Fatalf("pid %d: want one kill(%d, SIGKILL), got %d call(s) kill(%d, %v)", pid, -pid, calls, gotPid, gotSig)
		}
	}
}

func TestKillProcessGroup_PropagatesKillError(t *testing.T) {
	want := syscall.ESRCH
	err := killProcessGroup(4242, syscall.SIGKILL, func(int, syscall.Signal) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
}
