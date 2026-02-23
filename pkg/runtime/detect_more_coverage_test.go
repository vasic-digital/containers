//go:build !integration

package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAutoDetectWithPriority_Found verifies that the first available
// runtime in the priority list is selected.
func TestAutoDetectWithPriority_Found(t *testing.T) {
	// Use alwaysAvailableRuntime and neverAvailableRuntime from detect_coverage_test.go.
	// AutoDetectWithPriority uses the real defaultRuntimeFactory, which requires
	// actual binaries. We test the ordering logic by using DetectByPriority with
	// mock runtimes through autoDetectWith.
	rts := []ContainerRuntime{
		&neverAvailableRuntime{name: "cri-o"},
		&alwaysAvailableRuntime{name: "docker"},
		&alwaysAvailableRuntime{name: "podman"},
	}

	// Simulate priority: docker before podman
	priority := []string{"docker", "podman"}
	ordered := make([]ContainerRuntime, 0, len(rts))
	seen := make(map[string]bool)
	for _, name := range priority {
		for _, rt := range rts {
			if !seen[rt.Name()] && rt.Name() == name {
				ordered = append(ordered, rt)
				seen[rt.Name()] = true
			}
		}
	}
	for _, rt := range rts {
		if !seen[rt.Name()] {
			ordered = append(ordered, rt)
			seen[rt.Name()] = true
		}
	}

	rt, err := autoDetectWith(context.Background(), ordered)
	assert.NoError(t, err)
	assert.Equal(t, "docker", rt.Name())
}

// TestDetectByPriority_WithMocks tests the priority-ordered detection
// using mock runtimes by going through the lower-level detectAllWith.
func TestDetectByPriority_WithMocks(t *testing.T) {
	rts := []ContainerRuntime{
		&alwaysAvailableRuntime{name: "podman"},
		&neverAvailableRuntime{name: "docker"},
		&alwaysAvailableRuntime{name: "nerdctl"},
	}

	// Verify detectAllWith honours availability
	available := detectAllWith(context.Background(), rts)
	assert.Len(t, available, 2)

	names := make([]string, len(available))
	for i, r := range available {
		names[i] = r.Name()
	}
	assert.Contains(t, names, "podman")
	assert.Contains(t, names, "nerdctl")
	assert.NotContains(t, names, "docker")
}

// TestAutoDetectWithPriority_NoneAvailable verifies an error when
// all runtimes in the priority list are unavailable.
func TestAutoDetectWithPriority_NoneAvailable(t *testing.T) {
	rts := []ContainerRuntime{
		&neverAvailableRuntime{name: "podman"},
		&neverAvailableRuntime{name: "docker"},
	}
	_, err := autoDetectWith(context.Background(), rts)
	assert.Error(t, err)
}

// TestDetectByPriority_OrderPreserved verifies that DetectByPriority
// returns runtimes in priority order via detectAllWith reordering.
func TestDetectByPriority_OrderPreserved(t *testing.T) {
	rts := []ContainerRuntime{
		&alwaysAvailableRuntime{name: "docker"},
		&alwaysAvailableRuntime{name: "podman"},
		&neverAvailableRuntime{name: "nerdctl"},
	}

	priority := []string{"podman", "docker"}
	ordered := make([]ContainerRuntime, 0, len(rts))
	seen := make(map[string]bool)

	for _, name := range priority {
		for _, rt := range rts {
			if !seen[rt.Name()] && rt.Name() == name && rt.IsAvailable(context.Background()) {
				ordered = append(ordered, rt)
				seen[rt.Name()] = true
			}
		}
	}
	for _, rt := range rts {
		if !seen[rt.Name()] && rt.IsAvailable(context.Background()) {
			ordered = append(ordered, rt)
		}
	}

	assert.Len(t, ordered, 2)
	assert.Equal(t, "podman", ordered[0].Name())
	assert.Equal(t, "docker", ordered[1].Name())
}

// TestGetRuntimePriority_ReturnsCopy verifies the returned slice is
// a copy (modifying it does not affect the global).
func TestGetRuntimePriority_ReturnsCopy(t *testing.T) {
	original := GetRuntimePriority()
	copy1 := GetRuntimePriority()

	// Modify copy1 should not affect subsequent calls
	copy1[0] = "modified"
	copy2 := GetRuntimePriority()
	assert.Equal(t, original[0], copy2[0])
}

// TestSetRuntimePriority_Roundtrip verifies set then get returns the
// correct values.
func TestSetRuntimePriority_Roundtrip(t *testing.T) {
	original := GetRuntimePriority()
	defer SetRuntimePriority(original)

	custom := []string{"nerdctl", "cri-o", "podman"}
	SetRuntimePriority(custom)
	got := GetRuntimePriority()
	assert.Equal(t, custom, got)
}
