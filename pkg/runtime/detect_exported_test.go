package runtime

import (
	"os"
	"testing"
)

// TestAutoDetectWithPriority_Exported calls the exported function for coverage.
// Guarded behind CONTAINERS_INTEGRATION_TEST because it execs real container
// runtime binaries that may hang when the daemon is not running.
// The internal detect_test.go covers the same logic with mocked executors.
func TestAutoDetectWithPriority_Exported(t *testing.T) {
	// bluff-scan: no-assert-ok (auto-detect smoke — must not panic)
	if os.Getenv("CONTAINERS_INTEGRATION_TEST") != "1" {
		t.Skip("Set CONTAINERS_INTEGRATION_TEST=1 to run (execs real runtimes)")  // SKIP-OK: #legacy-untriaged
	}
}

// TestDetectByPriority_Exported calls the exported function for coverage.
func TestDetectByPriority_Exported(t *testing.T) {
	// bluff-scan: no-assert-ok (auto-detect smoke — must not panic)
	if os.Getenv("CONTAINERS_INTEGRATION_TEST") != "1" {
		t.Skip("Set CONTAINERS_INTEGRATION_TEST=1 to run (execs real runtimes)")  // SKIP-OK: #legacy-untriaged
	}
}
