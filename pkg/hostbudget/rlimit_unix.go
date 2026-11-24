//go:build linux || darwin || freebsd

package hostbudget

import "golang.org/x/sys/unix"

// rlimitNproc reads RLIMIT_NPROC with the constants of golang.org/x/sys/unix
// (linux 6, darwin/freebsd 7; RLIM_INFINITY differs per OS: ^0 on linux,
// 0x7fffffffffffffff on darwin/freebsd) instead of hand-copied literals.
func rlimitNproc() (uint64, bool, error) {
	var rl unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rl); err != nil {
		return 0, false, err
	}
	if uint64(rl.Cur) == uint64(unix.RLIM_INFINITY) {
		return 0, false, nil
	}
	return uint64(rl.Cur), true, nil
}
