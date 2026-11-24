//go:build unix

package remote

import (
	"os"
	"syscall"
	"testing"
)

// TestMain pins the process umask to 0022 for this package's tests.
//
// The pool's hardening refuses a pre-existing control-socket directory that is
// group/other-writable, and several tests use t.TempDir(), which is created
// 0777 &^ umask. Under the very common umask 0002 that is 0775, so the tests
// failed (and TestNewConnectionPool panicked on a nil pool, aborting the whole
// package) for an environmental reason, not a product defect. Pinning the
// umask makes the suite hermetic; production behaviour is unchanged.
func TestMain(m *testing.M) {
	syscall.Umask(0o022)
	os.Exit(m.Run())
}
