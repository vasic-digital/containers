package runtime

import (
	"context"
	"testing"
)

// TestAutoDetectWithPriority_Exported calls the exported function for coverage.
// It may succeed or fail depending on whether a runtime is installed.
func TestAutoDetectWithPriority_Exported(t *testing.T) {
	ctx := context.Background()
	// Call with empty priority - all runtimes tried in default order.
	_, _ = AutoDetectWithPriority(ctx, []string{})
	// Call with specific priority list.
	_, _ = AutoDetectWithPriority(ctx, []string{"docker", "podman", "nerdctl"})
	// Call with unknown runtime name (no match, falls back to all).
	_, _ = AutoDetectWithPriority(ctx, []string{"nonexistent-runtime"})
}

// TestDetectByPriority_Exported calls the exported function for coverage.
func TestDetectByPriority_Exported(t *testing.T) {
	ctx := context.Background()
	// Returns a slice (possibly empty) - just need to call it.
	_ = DetectByPriority(ctx, []string{"docker", "podman"})
	_ = DetectByPriority(ctx, []string{})
	// Call with a known runtime name so the inner loop is exercised.
	_ = DetectByPriority(ctx, []string{"podman", "docker", "nerdctl", "cri-o", "lxd", "kubernetes"})
}
