package monitor_test

import (
	"testing"

	"digital.vasic.containers/internal/platform"
	"digital.vasic.containers/pkg/monitor"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSystemCollector_Collect(t *testing.T) {
	c := monitor.NewDefaultSystemCollector()
	res := c.Collect()

	if platform.IsLinux() {
		// On Linux we should get real memory data.
		assert.True(t, res.MemoryTotal > 0,
			"expected non-zero MemoryTotal on Linux")
		assert.True(t, res.MemoryUsed > 0,
			"expected non-zero MemoryUsed on Linux")
		assert.True(t, res.MemoryPercent > 0,
			"expected non-zero MemoryPercent on Linux")
	} else {
		// On other platforms we still expect Go runtime data.
		assert.True(t, res.MemoryTotal > 0,
			"expected non-zero MemoryTotal from Go runtime")
	}
}

func TestDefaultSystemCollector_CollectTwice(t *testing.T) {
	c := monitor.NewDefaultSystemCollector()

	// First and second calls should both succeed without panics.
	res1 := c.Collect()
	res2 := c.Collect()

	assert.True(t, res1.MemoryTotal > 0)
	assert.True(t, res2.MemoryTotal > 0)
}

func TestStubSystemCollector_Implements(t *testing.T) {
	var _ monitor.SystemCollector = &stubSystemCollector{}
}

// TestDefaultSystemCollector_CPUBounds verifies CPU percentage is within valid
// range
func TestDefaultSystemCollector_CPUBounds(t *testing.T) {
	if !platform.IsLinux() {
		t.Skip("CPU collection only supported on Linux")
	}

	c := monitor.NewDefaultSystemCollector()
	res := c.Collect()

	// CPU percent should be between 0 and 100
	assert.GreaterOrEqual(t, res.CPUPercent, 0.0,
		"CPU percent should be non-negative")
	assert.LessOrEqual(t, res.CPUPercent, 100.0,
		"CPU percent should not exceed 100")
}

// TestDefaultSystemCollector_MemoryBounds verifies memory percentage is within
// valid range
func TestDefaultSystemCollector_MemoryBounds(t *testing.T) {
	c := monitor.NewDefaultSystemCollector()
	res := c.Collect()

	// Memory percent should be between 0 and 100
	assert.GreaterOrEqual(t, res.MemoryPercent, 0.0,
		"Memory percent should be non-negative")
	assert.LessOrEqual(t, res.MemoryPercent, 100.0,
		"Memory percent should not exceed 100")

	// Used should not exceed total
	assert.LessOrEqual(t, res.MemoryUsed, res.MemoryTotal,
		"Memory used should not exceed total")
}

// TestDefaultSystemCollector_MultipleCollections tests rapid successive
// collections
func TestDefaultSystemCollector_MultipleCollections(t *testing.T) {
	c := monitor.NewDefaultSystemCollector()

	// Collect multiple times in rapid succession
	for i := 0; i < 5; i++ {
		res := c.Collect()
		assert.True(t, res.MemoryTotal > 0,
			"expected non-zero MemoryTotal on iteration %d", i)
	}
}

// TestDefaultSystemCollector_CollectValidation verifies the collector returns
// consistent data
func TestDefaultSystemCollector_CollectValidation(t *testing.T) {
	c := monitor.NewDefaultSystemCollector()

	res := c.Collect()

	// MemoryTotal should be consistent (same machine)
	res2 := c.Collect()
	assert.Equal(t, res.MemoryTotal, res2.MemoryTotal,
		"MemoryTotal should be consistent across collections")
}
