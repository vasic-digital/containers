//go:build !linux && !darwin && !freebsd

package hostbudget

import "errors"

func rlimitNproc() (uint64, bool, error) {
	return 0, false, errors.New("RLIMIT_NPROC probe not implemented on this OS")
}
