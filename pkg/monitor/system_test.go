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
