package discovery

import (
	"context"
	"time"
)

// DiscoveryTarget describes a service endpoint to discover.
type DiscoveryTarget struct {
	// Name is a human-readable identifier for the target.
	Name string
	// Host is the hostname or IP address to probe.
	Host string
	// Port is the port number to probe.
	Port string
	// Method is the discovery mechanism ("tcp", "dns").
	Method string
	// Timeout is the maximum duration for a discovery attempt.
	Timeout time.Duration
}

// Discoverer checks whether a service endpoint is reachable using
// the configured discovery mechanism.
type Discoverer interface {
	// Discover probes the target and returns true when the
	// service is reachable, or false with an error when it is
	// not.
	Discover(ctx context.Context, target DiscoveryTarget) (bool, error)
}
