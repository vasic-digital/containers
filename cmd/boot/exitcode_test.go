package main

import (
	"fmt"
	"testing"

	"digital.vasic.containers/pkg/runtime"
)

// A forced runner that is not installed must exit 2 like an unknown one (docs/RUNNER_FLAG.md), not 1.
func TestExitCodeForForcedRunnerRefusal(t *testing.T) {
	wrapped := fmt.Errorf("select runtime: %w", fmt.Errorf("no container runtime: %w", runtime.ErrRunnerUnavailable))
	if got := exitCodeFor(wrapped); got != 2 {
		t.Fatalf("unavailable forced runner: exit %d want 2", got)
	}
	if got := exitCodeFor(fmt.Errorf("load env config: boom")); got != 1 {
		t.Fatalf("ordinary failure: exit %d want 1", got)
	}
}
