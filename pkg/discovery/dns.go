package discovery

import (
	"context"
	"fmt"
	"net"
	"time"
)

// hostLookup defines the interface for DNS host lookups.
type hostLookup interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// defaultHostLookup uses net.Resolver for DNS lookups.
type defaultHostLookup struct {
	resolver *net.Resolver
}

func (d *defaultHostLookup) LookupHost(
	ctx context.Context, host string,
) ([]string, error) {
	return d.resolver.LookupHost(ctx, host)
}

// DNSDiscoverer implements Discoverer by performing a DNS lookup
// for the target host.
type DNSDiscoverer struct {
	lookup hostLookup
}

// NewDNSDiscoverer returns a new DNSDiscoverer.
func NewDNSDiscoverer() *DNSDiscoverer {
	return &DNSDiscoverer{
		lookup: &defaultHostLookup{resolver: &net.Resolver{}},
	}
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

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	lookup := d.lookup
	if lookup == nil {
		lookup = &defaultHostLookup{resolver: &net.Resolver{}}
	}

	addrs, err := lookup.LookupHost(ctx, target.Host)
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
