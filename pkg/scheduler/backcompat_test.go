package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/remote"
)

// TestBackCompat_NoGPU_NoChange asserts the pre-GPU-extension
// behaviour is unchanged: a host with no GPU + a requirement with
// no GPU schedules exactly as before.
func TestBackCompat_NoGPU_NoChange(t *testing.T) {
	s := NewResourceScorer(Options{
		ReservePercent:  0,
		OvercommitRatio: 1,
		CPUWeight:       0.5,
		MemoryWeight:    0.5,
	})
	res := &remote.HostResources{
		Host:          "legacy",
		CPUCores:      4,
		MemoryTotalMB: 8_000,
	}
	req := ContainerRequirements{Name: "nginx", CPUCores: 0.5, MemoryMB: 256}
	require.True(t, s.CanFit(res, req))
	require.Greater(t, s.Score(res, req), 0.0)
}
