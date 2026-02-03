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

// TestNoopCollector_IndividualMethods tests each NoopCollector method
// individually to ensure 100% coverage of the noop.go file.
func TestNoopCollector_IndividualMethods(t *testing.T) {
	tests := []struct {
		name string
		fn   func(c NoopCollector)
	}{
		{
			name: "IncContainerStarts",
			fn: func(c NoopCollector) {
				c.IncContainerStarts("test-container")
			},
		},
		{
			name: "IncContainerStops",
			fn: func(c NoopCollector) {
				c.IncContainerStops("test-container")
			},
		},
		{
			name: "IncContainerFailures",
			fn: func(c NoopCollector) {
				c.IncContainerFailures("test-container")
			},
		},
		{
			name: "ObserveHealthCheckDuration",
			fn: func(c NoopCollector) {
				c.ObserveHealthCheckDuration("test-container", 100*time.Millisecond)
			},
		},
		{
			name: "ObserveBootDuration",
			fn: func(c NoopCollector) {
				c.ObserveBootDuration(5 * time.Second)
			},
		},
		{
			name: "SetContainerUp_true",
			fn: func(c NoopCollector) {
				c.SetContainerUp("test-container", true)
			},
		},
		{
			name: "SetContainerUp_false",
			fn: func(c NoopCollector) {
				c.SetContainerUp("test-container", false)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NoopCollector{}
			// Should not panic
			assert.NotPanics(t, func() {
				tt.fn(c)
			})
		})
	}
}

// TestNoopCollector_EmptyContainerName tests NoopCollector with empty names.
func TestNoopCollector_EmptyContainerName(t *testing.T) {
	c := &NoopCollector{}

	// All methods should handle empty names without panicking.
	assert.NotPanics(t, func() {
		c.IncContainerStarts("")
		c.IncContainerStops("")
		c.IncContainerFailures("")
		c.ObserveHealthCheckDuration("", time.Millisecond)
		c.SetContainerUp("", true)
	})
}

// TestNoopCollector_ZeroDuration tests NoopCollector with zero durations.
func TestNoopCollector_ZeroDuration(t *testing.T) {
	c := &NoopCollector{}

	assert.NotPanics(t, func() {
		c.ObserveHealthCheckDuration("svc", 0)
		c.ObserveBootDuration(0)
	})
}

// TestNoopCollector_NegativeDuration tests NoopCollector with negative
// durations.
func TestNoopCollector_NegativeDuration(t *testing.T) {
	c := &NoopCollector{}

	assert.NotPanics(t, func() {
		c.ObserveHealthCheckDuration("svc", -time.Second)
		c.ObserveBootDuration(-time.Second)
	})
}
