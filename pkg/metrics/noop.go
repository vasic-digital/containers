package metrics

import "time"

// NoopCollector is a MetricsCollector that silently discards all
// metric recordings. Use it when metrics collection is not
// required.
type NoopCollector struct{}

// Ensure NoopCollector satisfies MetricsCollector at compile
// time.
var _ MetricsCollector = (*NoopCollector)(nil)

// IncContainerStarts is a no-op.
func (NoopCollector) IncContainerStarts(string) {}

// IncContainerStops is a no-op.
func (NoopCollector) IncContainerStops(string) {}

// IncContainerFailures is a no-op.
func (NoopCollector) IncContainerFailures(string) {}

// ObserveHealthCheckDuration is a no-op.
func (NoopCollector) ObserveHealthCheckDuration(string, time.Duration) {}

// ObserveBootDuration is a no-op.
func (NoopCollector) ObserveBootDuration(time.Duration) {}

// SetContainerUp is a no-op.
func (NoopCollector) SetContainerUp(string, bool) {}
