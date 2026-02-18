package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"digital.vasic.containers/pkg/remote"
)

func TestResourceScorer_Score_HighAvailability(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())

	res := &remote.HostResources{
		CPUPercent:    20,
		MemoryPercent: 20,
		MemoryTotalMB: 32768,
		DiskPercent:   30,
		DiskTotalMB:   500000,
		CPUCores:      16,
	}

	score := scorer.Score(res, ContainerRequirements{})
	assert.Greater(t, score, 0.5)
	assert.LessOrEqual(t, score, 1.0)
}

func TestResourceScorer_Score_LowAvailability(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())

	res := &remote.HostResources{
		CPUPercent:    95,
		MemoryPercent: 95,
		MemoryTotalMB: 8192,
		DiskPercent:   95,
		DiskTotalMB:   100000,
		CPUCores:      4,
	}

	score := scorer.Score(res, ContainerRequirements{})
	assert.LessOrEqual(t, score, 0.1)
}

func TestResourceScorer_Score_NilResources(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())
	score := scorer.Score(nil, ContainerRequirements{})
	assert.Equal(t, 0.0, score)
}

func TestResourceScorer_CanFit_Sufficient(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())

	res := &remote.HostResources{
		CPUPercent:    20,
		MemoryPercent: 20,
		MemoryTotalMB: 32768,
		DiskPercent:   30,
		DiskTotalMB:   500000,
		CPUCores:      16,
	}

	req := ContainerRequirements{
		CPUCores: 2,
		MemoryMB: 4096,
		DiskMB:   10000,
	}

	assert.True(t, scorer.CanFit(res, req))
}

func TestResourceScorer_CanFit_InsufficientMemory(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())

	res := &remote.HostResources{
		CPUPercent:    20,
		MemoryPercent: 90,
		MemoryTotalMB: 8192,
		DiskPercent:   30,
		DiskTotalMB:   500000,
		CPUCores:      8,
	}

	req := ContainerRequirements{
		MemoryMB: 4096,
	}

	assert.False(t, scorer.CanFit(res, req))
}

func TestResourceScorer_CanFit_InsufficientCPU(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())

	res := &remote.HostResources{
		CPUPercent:    95,
		MemoryPercent: 20,
		MemoryTotalMB: 32768,
		DiskPercent:   30,
		DiskTotalMB:   500000,
		CPUCores:      4,
	}

	req := ContainerRequirements{
		CPUCores: 2,
	}

	assert.False(t, scorer.CanFit(res, req))
}

func TestResourceScorer_CanFit_InsufficientDisk(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())

	res := &remote.HostResources{
		CPUPercent:    20,
		MemoryPercent: 20,
		MemoryTotalMB: 32768,
		DiskPercent:   95,
		DiskTotalMB:   100000,
		CPUCores:      16,
	}

	req := ContainerRequirements{
		DiskMB: 50000,
	}

	assert.False(t, scorer.CanFit(res, req))
}

func TestResourceScorer_CanFit_NilResources(t *testing.T) {
	scorer := NewResourceScorer(DefaultSchedulerOptions())
	assert.False(t, scorer.CanFit(nil, ContainerRequirements{}))
}

func TestResourceScorer_CanFit_WithOvercommit(t *testing.T) {
	opts := DefaultSchedulerOptions()
	opts.OvercommitRatio = 2.0
	scorer := NewResourceScorer(opts)

	res := &remote.HostResources{
		CPUPercent:    60,
		MemoryPercent: 60,
		MemoryTotalMB: 16384,
		DiskPercent:   30,
		DiskTotalMB:   500000,
		CPUCores:      8,
	}

	// Without overcommit this wouldn't fit.
	req := ContainerRequirements{
		MemoryMB: 8000,
	}

	assert.True(t, scorer.CanFit(res, req))
}

func TestClamp(t *testing.T) {
	assert.Equal(t, 0.0, clamp(-1.0, 0, 1))
	assert.Equal(t, 1.0, clamp(2.0, 0, 1))
	assert.Equal(t, 0.5, clamp(0.5, 0, 1))
}
