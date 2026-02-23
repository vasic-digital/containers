package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"digital.vasic.containers/pkg/remote"
)

// TestScoreMemory_RequestTooLarge exercises the inner `uint64(availMB) <
// req.MemoryMB` branch inside scoreMemory. The host has enough free
// percentage but the absolute megabytes available are less than the
// requested amount, so the function returns 0.
func TestScoreMemory_RequestTooLarge(t *testing.T) {
	opts := DefaultSchedulerOptions()
	opts.ReservePercent = 0
	opts.OvercommitRatio = 1.0
	s := NewResourceScorer(opts)

	// Host: 100 MB total, 20% used → 80 MB available.
	// Requesting 8192 MB → availMB (80) < MemoryMB (8192) → return 0.
	res := &remote.HostResources{
		MemoryPercent: 20,
		MemoryTotalMB: 100,
	}
	score := s.scoreMemory(
		res,
		ContainerRequirements{MemoryMB: 8192},
		0,
		1.0,
	)
	assert.Equal(t, 0.0, score,
		"scoreMemory should return 0 when availMB < req.MemoryMB")
}

// TestScoreMemory_ZeroTotal_WithRequest exercises the path where
// MemoryTotalMB is 0 with a non-zero MemoryMB request. The inner
// `if req.MemoryMB > 0 && r.MemoryTotalMB > 0` guard evaluates to
// false (because TotalMB == 0), so the function falls through to the
// plain avail/100 score without entering the request-specific block.
func TestScoreMemory_ZeroTotal_WithRequest(t *testing.T) {
	opts := DefaultSchedulerOptions()
	opts.ReservePercent = 0
	opts.OvercommitRatio = 1.0
	s := NewResourceScorer(opts)

	// MemoryTotalMB = 0 means the "MemoryTotalMB > 0" guard is false,
	// so the req block is skipped. avail = 100 - 50 = 50, score = 0.5.
	res := &remote.HostResources{
		MemoryPercent: 50,
		MemoryTotalMB: 0,
	}
	score := s.scoreMemory(
		res,
		ContainerRequirements{MemoryMB: 100},
		0,
		1.0,
	)
	assert.InDelta(t, 0.5, score, 0.001,
		"scoreMemory should fall back to avail/100 when MemoryTotalMB is 0")
}
