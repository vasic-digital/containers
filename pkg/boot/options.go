package boot

import (
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/discovery"
	"digital.vasic.containers/pkg/event"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/metrics"
	"digital.vasic.containers/pkg/runtime"
)

// BootManagerOption configures a BootManager during construction.
type BootManagerOption func(*BootManager)

// WithRuntime sets the container runtime.
func WithRuntime(r runtime.ContainerRuntime) BootManagerOption {
	return func(bm *BootManager) {
		bm.runtime = r
	}
}

// WithLogger sets the logger.
func WithLogger(l logging.Logger) BootManagerOption {
	return func(bm *BootManager) {
		bm.logger = l
	}
}

// WithMetrics sets the metrics collector.
func WithMetrics(m metrics.MetricsCollector) BootManagerOption {
	return func(bm *BootManager) {
		bm.metrics = m
	}
}

// WithEventBus sets the event bus.
func WithEventBus(bus event.EventBus) BootManagerOption {
	return func(bm *BootManager) {
		bm.eventBus = bus
	}
}

// WithOrchestrator sets the compose orchestrator.
func WithOrchestrator(
	o compose.ComposeOrchestrator,
) BootManagerOption {
	return func(bm *BootManager) {
		bm.orchestrator = o
	}
}

// WithHealthChecker sets the health checker.
func WithHealthChecker(h health.HealthChecker) BootManagerOption {
	return func(bm *BootManager) {
		bm.healthChecker = h
	}
}

// WithProjectDir sets the project root directory used for
// resolving relative compose file paths.
func WithProjectDir(dir string) BootManagerOption {
	return func(bm *BootManager) {
		bm.projectDir = dir
	}
}

// WithDiscoverer sets the service discoverer.
func WithDiscoverer(d discovery.Discoverer) BootManagerOption {
	return func(bm *BootManager) {
		bm.discoverer = d
	}
}
