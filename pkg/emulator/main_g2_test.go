package emulator

import (
	"os"
	"testing"

	"digital.vasic.containers/pkg/hostbudget"
)

// TestMain makes the legacy Boot unit tests independent of the machine they run
// on: the DEFAULT admitter (which reads the live /proc) is replaced by a
// permissive one for the whole package. The G2 tests set Admit explicitly and
// the real-host smoke test in pkg/hostbudget covers the default path.
func TestMain(m *testing.M) {
	defaultAdmit = func(hostbudget.Need) hostbudget.Evidence {
		return hostbudget.Evidence{Verdict: hostbudget.Fits}
	}
	os.Exit(m.Run())
}
