package monitor

import "time"

// ResourceSnapshot captures a point-in-time view of system and
// container resource usage.
type ResourceSnapshot struct {
	// Timestamp is when the snapshot was taken.
	Timestamp time.Time
	// System holds host-level resource metrics.
	System SystemResources
	// Containers holds per-container resource metrics keyed by
	// container name.
	Containers map[string]ContainerResources
}

// SystemResources holds host-level CPU, memory, and disk metrics.
type SystemResources struct {
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
	MemoryTotal   uint64
	MemoryUsed    uint64
	DiskTotal     uint64
	DiskUsed      uint64
}

// ContainerResources holds resource usage for a single container.
type ContainerResources struct {
	Name          string
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsage   uint64
	MemoryLimit   uint64
}

// ThresholdRule defines a condition on a metric that, when met,
// triggers the associated action.
type ThresholdRule struct {
	// Metric is the name of the metric to evaluate
	// (e.g., "system.cpu", "system.memory",
	// "container.<name>.memory").
	Metric string
	// Threshold is the numeric boundary.
	Threshold float64
	// Operator is the comparison operator: ">", ">=", "<", "<=".
	Operator string
	// Action is called when the threshold condition is met.
	Action func(snapshot *ResourceSnapshot)
}
