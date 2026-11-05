//go:build unix

package crossbuild

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// errUnsafeProcessGroup is returned when a process-group signal is
// refused because the pid is <= 1 (constitution §11.4.263): kill(-1, sig)
// signals every process the caller may signal, kill(0, sig) signals the
// caller's own group, and kill(-pid) for pid <= 0 is never a child group.
var errUnsafeProcessGroup = errors.New("crossbuild: refusing process-group signal for pid <= 1")

// killProcessGroup sends sig to the process group led by pid (kill(-pid,
// sig)) through kill, and refuses — without calling kill — when pid <= 1.
func killProcessGroup(pid int, sig syscall.Signal, kill func(int, syscall.Signal) error) error {
	if pid <= 1 {
		return fmt.Errorf("%w (pid=%d)", errUnsafeProcessGroup, pid)
	}
	return kill(-pid, sig)
}

// configureProcessGroup arranges for cmd's ENTIRE process group to be
// killed when its context is cancelled or its WaitDelay expires — not
// just the direct child. Without this, a build command that
// backgrounds a detached grandchild (a Gradle daemon, a `cmd & disown`
// shell job, a Wine server) leaves that grandchild running after
// Run() returns: the default exec.Cmd.Cancel behaviour only signals
// cmd.Process (the immediate "sh" child), and a background job that
// merely disowned itself from job control — without calling setsid —
// stays in the SAME process group and is simply reparented (not
// killed) when its parent dies.
//
// Setpgid:true puts the child in its OWN new process group (pgid ==
// the child's own pid) instead of inheriting the CALLER's pgid, so a
// negative-pid kill(2) targets EXACTLY this command's own descendants
// and can never reach the calling process's (or a sibling command's)
// process group.
//
// cmd.Cancel is invoked by the exec package's context-watch goroutine
// when ctx is done (or WaitDelay's grace period elapses) BEFORE the
// command has exited on its own; overriding it here (rather than
// leaving the default cmd.Process.Kill()) is what upgrades "kill one
// process" to "kill the whole group".
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// A negative pid targets the whole process GROUP per kill(2).
		// killProcessGroup refuses a pid <= 1 (§11.4.263). Such a pid is
		// never a started child, so nothing is signalled at all (not even
		// the single-process fallback, which for pid 1 would signal init).
		pid := cmd.Process.Pid
		if err := killProcessGroup(pid, syscall.SIGKILL, syscall.Kill); err != nil {
			if errors.Is(err, errUnsafeProcessGroup) {
				return err
			}
			// Fall back to killing just the direct child so Cancel
			// still makes forward progress even if, e.g., the group
			// has already exited (ESRCH) or Setpgid could not apply
			// for some other benign reason.
			return cmd.Process.Kill()
		}
		return nil
	}
}
