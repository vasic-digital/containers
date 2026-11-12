//go:build darwin

package hostbudget

import "syscall"

// rlimitNprocID is RLIMIT_NPROC on darwin (the syscall package does not export it).
const rlimitNprocID = 8

func rlimitNproc() (uint64, bool, error) {
	var rl syscall.Rlimit
	if err := syscall.Getrlimit(rlimitNprocID, &rl); err != nil {
		return 0, false, err
	}
	if rl.Cur == ^uint64(0) {
		return 0, false, nil
	}
	return rl.Cur, true, nil
}
