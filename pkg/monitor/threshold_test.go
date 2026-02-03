package monitor_test

import (
	"testing"
	"time"

	"digital.vasic.containers/pkg/monitor"

	"github.com/stretchr/testify/assert"
)

func TestThresholdEvaluator_AddRule(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	assert.Len(t, e.Rules(), 0)

	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 80,
		Operator:  ">",
	})
	assert.Len(t, e.Rules(), 1)
}

func TestThresholdEvaluator_Evaluate_SystemCPU(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		System:    monitor.SystemResources{CPUPercent: 75},
	}
	e.Evaluate(snap)
	assert.True(t, fired, "expected rule to fire for CPU 75 > 50")
}

func TestThresholdEvaluator_Evaluate_NotTriggered(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 90,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		System:    monitor.SystemResources{CPUPercent: 50},
	}
	e.Evaluate(snap)
	assert.False(t, fired, "expected rule not to fire for CPU 50 > 90")
}

func TestThresholdEvaluator_Evaluate_ContainerMetric(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "container.redis.memory",
		Threshold: 80,
		Operator:  ">=",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		Containers: map[string]monitor.ContainerResources{
			"redis": {
				Name:          "redis",
				MemoryPercent: 85,
			},
		},
	}
	e.Evaluate(snap)
	assert.True(t, fired)
}

func TestThresholdEvaluator_Evaluate_LessThan(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.memory",
		Threshold: 20,
		Operator:  "<",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		System:    monitor.SystemResources{MemoryPercent: 10},
	}
	e.Evaluate(snap)
	assert.True(t, fired)
}

func TestThresholdEvaluator_Evaluate_NilSnapshot(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			t.Fatal("should not be called on nil snapshot")
		},
	})
	// Should not panic.
	e.Evaluate(nil)
}

func TestThresholdEvaluator_Evaluate_UnknownMetric(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "unknown.metric",
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{Timestamp: time.Now()}
	e.Evaluate(snap)
	assert.False(t, fired)
}

func TestThresholdEvaluator_Evaluate_Operators(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		op       string
		thresh   float64
		expected bool
	}{
		{"gt_true", 90, ">", 80, true},
		{"gt_false", 80, ">", 80, false},
		{"gte_true", 80, ">=", 80, true},
		{"gte_false", 79, ">=", 80, false},
		{"lt_true", 10, "<", 20, true},
		{"lt_false", 20, "<", 20, false},
		{"lte_true", 20, "<=", 20, true},
		{"lte_false", 21, "<=", 20, false},
		{"eq_true", 50, "==", 50, true},
		{"eq_false", 49, "==", 50, false},
		{"invalid_op", 50, "!=", 50, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := monitor.NewThresholdEvaluator()
			fired := false
			e.AddRule(monitor.ThresholdRule{
				Metric:    "system.cpu",
				Threshold: tc.thresh,
				Operator:  tc.op,
				Action: func(_ *monitor.ResourceSnapshot) {
					fired = true
				},
			})
			snap := &monitor.ResourceSnapshot{
				Timestamp: time.Now(),
				System: monitor.SystemResources{
					CPUPercent: tc.value,
				},
			}
			e.Evaluate(snap)
			assert.Equal(t, tc.expected, fired)
		})
	}
}

// TestThresholdEvaluator_Evaluate_SystemDisk tests system.disk metric
func TestThresholdEvaluator_Evaluate_SystemDisk(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.disk",
		Threshold: 70,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		System:    monitor.SystemResources{DiskPercent: 80},
	}
	e.Evaluate(snap)
	assert.True(t, fired, "expected rule to fire for disk 80 > 70")
}

// TestThresholdEvaluator_Evaluate_ContainerCPU tests container.<name>.cpu
// metric
func TestThresholdEvaluator_Evaluate_ContainerCPU(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "container.nginx.cpu",
		Threshold: 50,
		Operator:  ">=",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		Containers: map[string]monitor.ContainerResources{
			"nginx": {
				Name:       "nginx",
				CPUPercent: 75,
			},
		},
	}
	e.Evaluate(snap)
	assert.True(t, fired)
}

// TestThresholdEvaluator_Evaluate_ContainerNotFound tests when container
// doesn't exist
func TestThresholdEvaluator_Evaluate_ContainerNotFound(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "container.nonexistent.cpu",
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp:  time.Now(),
		Containers: map[string]monitor.ContainerResources{},
	}
	e.Evaluate(snap)
	assert.False(t, fired, "rule should not fire for nonexistent container")
}

// TestThresholdEvaluator_Evaluate_ContainerUnknownField tests unknown
// container field
func TestThresholdEvaluator_Evaluate_ContainerUnknownField(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "container.redis.disk", // disk is not a valid field
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		Containers: map[string]monitor.ContainerResources{
			"redis": {Name: "redis", CPUPercent: 100},
		},
	}
	e.Evaluate(snap)
	assert.False(t, fired, "rule should not fire for unknown field")
}

// TestThresholdEvaluator_Evaluate_ContainerInvalidFormat tests malformed
// container metric
func TestThresholdEvaluator_Evaluate_ContainerInvalidFormat(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	fired := false
	e.AddRule(monitor.ThresholdRule{
		Metric:    "container.redis", // missing field part
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			fired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		Containers: map[string]monitor.ContainerResources{
			"redis": {Name: "redis", CPUPercent: 100},
		},
	}
	e.Evaluate(snap)
	assert.False(t, fired, "rule should not fire for invalid metric format")
}

// TestThresholdEvaluator_Evaluate_NilAction tests rule with nil action
func TestThresholdEvaluator_Evaluate_NilAction(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 0,
		Operator:  ">=",
		Action:    nil, // nil action
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		System:    monitor.SystemResources{CPUPercent: 50},
	}

	// Should not panic with nil action
	assert.NotPanics(t, func() {
		e.Evaluate(snap)
	})
}

// TestThresholdEvaluator_MultipleRules tests multiple rules firing
func TestThresholdEvaluator_MultipleRules(t *testing.T) {
	e := monitor.NewThresholdEvaluator()
	cpuFired := false
	memFired := false

	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 50,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			cpuFired = true
		},
	})
	e.AddRule(monitor.ThresholdRule{
		Metric:    "system.memory",
		Threshold: 60,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			memFired = true
		},
	})

	snap := &monitor.ResourceSnapshot{
		Timestamp: time.Now(),
		System: monitor.SystemResources{
			CPUPercent:    75,
			MemoryPercent: 70,
		},
	}
	e.Evaluate(snap)
	assert.True(t, cpuFired, "CPU rule should fire")
	assert.True(t, memFired, "Memory rule should fire")
}
