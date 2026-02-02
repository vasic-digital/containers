package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusCollector implements MetricsCollector using the
// Prometheus client library.
type PrometheusCollector struct {
	containerStarts   *prometheus.CounterVec
	containerStops    *prometheus.CounterVec
	containerFailures *prometheus.CounterVec
	healthCheckDur    *prometheus.HistogramVec
	bootDuration      prometheus.Histogram
	containerUp       *prometheus.GaugeVec
}

// Ensure PrometheusCollector satisfies MetricsCollector at
// compile time.
var _ MetricsCollector = (*PrometheusCollector)(nil)

// NewPrometheusCollector creates a PrometheusCollector and
// registers all metrics with the provided registerer. If reg is
// nil, prometheus.DefaultRegisterer is used.
func NewPrometheusCollector(
	reg prometheus.Registerer,
) *PrometheusCollector {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	c := &PrometheusCollector{
		containerStarts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "containers",
				Name:      "starts_total",
				Help:      "Total container start events.",
			},
			[]string{"name"},
		),
		containerStops: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "containers",
				Name:      "stops_total",
				Help:      "Total container stop events.",
			},
			[]string{"name"},
		),
		containerFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "containers",
				Name:      "failures_total",
				Help:      "Total container failure events.",
			},
			[]string{"name"},
		),
		healthCheckDur: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "containers",
				Name:      "health_check_duration_seconds",
				Help:      "Health check duration in seconds.",
				Buckets: prometheus.ExponentialBuckets(
					0.01, 2, 10,
				),
			},
			[]string{"name"},
		),
		bootDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "containers",
				Name:      "boot_duration_seconds",
				Help:      "Total boot sequence duration.",
				Buckets: prometheus.ExponentialBuckets(
					0.1, 2, 12,
				),
			},
		),
		containerUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "containers",
				Name:      "up",
				Help:      "Whether a container is up (1) or down (0).",
			},
			[]string{"name"},
		),
	}

	reg.MustRegister(
		c.containerStarts,
		c.containerStops,
		c.containerFailures,
		c.healthCheckDur,
		c.bootDuration,
		c.containerUp,
	)

	return c
}

// IncContainerStarts increments the start counter for the named
// container.
func (c *PrometheusCollector) IncContainerStarts(name string) {
	c.containerStarts.WithLabelValues(name).Inc()
}

// IncContainerStops increments the stop counter for the named
// container.
func (c *PrometheusCollector) IncContainerStops(name string) {
	c.containerStops.WithLabelValues(name).Inc()
}

// IncContainerFailures increments the failure counter for the
// named container.
func (c *PrometheusCollector) IncContainerFailures(name string) {
	c.containerFailures.WithLabelValues(name).Inc()
}

// ObserveHealthCheckDuration records the duration of a health
// check for the named container.
func (c *PrometheusCollector) ObserveHealthCheckDuration(
	name string,
	d time.Duration,
) {
	c.healthCheckDur.WithLabelValues(name).Observe(d.Seconds())
}

// ObserveBootDuration records the total boot sequence duration.
func (c *PrometheusCollector) ObserveBootDuration(d time.Duration) {
	c.bootDuration.Observe(d.Seconds())
}

// SetContainerUp sets the up gauge for the named container.
func (c *PrometheusCollector) SetContainerUp(
	name string,
	up bool,
) {
	val := 0.0
	if up {
		val = 1.0
	}
	c.containerUp.WithLabelValues(name).Set(val)
}
