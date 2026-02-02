package metrics

import "time"

// MetricsCollector defines the interface for recording container
// operational metrics.
type MetricsCollector interface {
	// IncContainerStarts increments the start counter for the
	// named container.
	IncContainerStarts(name string)
	// IncContainerStops increments the stop counter for the
	// named container.
	IncContainerStops(name string)
	// IncContainerFailures increments the failure counter for
	// the named container.
	IncContainerFailures(name string)
	// ObserveHealthCheckDuration records the duration of a
	// health check for the named container.
	ObserveHealthCheckDuration(name string, d time.Duration)
	// ObserveBootDuration records the total boot sequence
	// duration.
	ObserveBootDuration(d time.Duration)
	// SetContainerUp sets whether the named container is
	// currently up (true) or down (false).
	SetContainerUp(name string, up bool)
}
