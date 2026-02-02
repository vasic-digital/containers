package discovery

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DNSDiscoverer implements Discoverer by performing a DNS lookup
// for the target host.
type DNSDiscoverer struct{}

// NewDNSDiscoverer returns a new DNSDiscoverer.
func NewDNSDiscoverer() *DNSDiscoverer {
	return &DNSDiscoverer{}
}

// Discover performs a DNS lookup for target.Host and returns true
// when at least one address resolves.
func (d *DNSDiscoverer) Discover(
	ctx context.Context,
	target DiscoveryTarget,
) (bool, error) {
	if target.Host == "" {
		return false, fmt.Errorf(
			"discovery %s: host is required for DNS lookup",
			target.Name,
		)
	}

	timeout := target.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	addrs, err := resolver.LookupHost(ctx, target.Host)
	if err != nil {
		return false, fmt.Errorf(
			"discovery %s: dns lookup %s: %w",
			target.Name, target.Host, err,
		)
	}
	if len(addrs) == 0 {
		return false, fmt.Errorf(
			"discovery %s: dns lookup %s: no addresses found",
			target.Name, target.Host,
		)
	}
	return true, nil
}
