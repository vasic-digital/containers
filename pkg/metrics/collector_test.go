package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopCollector_ImplementsMetricsCollector(t *testing.T) {
	var c MetricsCollector = &NoopCollector{}
	require.NotNil(t, c)
}

func TestNoopCollector_AllMethods(t *testing.T) {
	c := &NoopCollector{}
	// Must not panic.
	c.IncContainerStarts("pg")
	c.IncContainerStops("pg")
	c.IncContainerFailures("pg")
	c.ObserveHealthCheckDuration("pg", 100*time.Millisecond)
	c.ObserveBootDuration(5 * time.Second)
	c.SetContainerUp("pg", true)
	c.SetContainerUp("pg", false)
}

func TestNoopCollector_MultipleContainers(t *testing.T) {
	c := &NoopCollector{}
	names := []string{"pg", "redis", "chromadb", "mock-llm"}
	for _, name := range names {
		c.IncContainerStarts(name)
		c.IncContainerStops(name)
		c.IncContainerFailures(name)
		c.ObserveHealthCheckDuration(name, time.Millisecond)
		c.SetContainerUp(name, true)
	}
}

func TestMetricsCollector_InterfaceCompliance(t *testing.T) {
	// Verify both implementations satisfy the interface.
	implementations := []MetricsCollector{
		&NoopCollector{},
	}
	for _, impl := range implementations {
		assert.NotNil(t, impl)
	}
}
