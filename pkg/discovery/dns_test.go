package discovery_test

import (
	"context"
	"testing"
	"time"

	"digital.vasic.containers/pkg/discovery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSDiscoverer_Discover_Success(t *testing.T) {
	tests := []struct {
		name   string
		target discovery.DiscoveryTarget
	}{
		{
			name: "localhost resolves",
			target: discovery.DiscoveryTarget{
				Name:    "local",
				Host:    "localhost",
				Method:  "dns",
				Timeout: 5 * time.Second,
			},
		},
		{
			name: "google.com resolves",
			target: discovery.DiscoveryTarget{
				Name:    "google",
				Host:    "google.com",
				Method:  "dns",
				Timeout: 5 * time.Second,
			},
		},
		{
			name: "IP address resolves as hostname",
			target: discovery.DiscoveryTarget{
				Name:    "loopback-ip",
				Host:    "127.0.0.1",
				Method:  "dns",
				Timeout: 5 * time.Second,
			},
		},
	}

	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := d.Discover(ctx, tt.target)
			require.NoError(t, err)
			assert.True(t, found)
		})
	}
}

func TestDNSDiscoverer_Discover_EmptyHost(t *testing.T) {
	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "empty-host",
		Host:    "",
		Method:  "dns",
		Timeout: time.Second,
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host is required for DNS lookup")
	assert.Contains(t, err.Error(), "empty-host")
}

func TestDNSDiscoverer_Discover_NonExistentDomain(t *testing.T) {
	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "nonexistent",
		Host:    "this-domain-definitely-does-not-exist-12345.invalid",
		Method:  "dns",
		Timeout: 5 * time.Second,
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dns lookup")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestDNSDiscoverer_Discover_CancelledContext(t *testing.T) {
	d := discovery.NewDNSDiscoverer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	target := discovery.DiscoveryTarget{
		Name:    "cancelled",
		Host:    "google.com",
		Method:  "dns",
		Timeout: 5 * time.Second,
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
}

func TestDNSDiscoverer_Discover_DefaultTimeout(t *testing.T) {
	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()

	// Target without explicit timeout - should use default 5s
	target := discovery.DiscoveryTarget{
		Name:   "default-timeout",
		Host:   "localhost",
		Method: "dns",
		// Timeout not set, should default to 5s
	}

	found, err := d.Discover(ctx, target)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestDNSDiscoverer_Discover_TimeoutExpires(t *testing.T) {
	d := discovery.NewDNSDiscoverer()

	// Use a context that will timeout very quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the context time to expire
	time.Sleep(10 * time.Millisecond)

	target := discovery.DiscoveryTarget{
		Name:    "timeout-test",
		Host:    "google.com",
		Method:  "dns",
		Timeout: 1 * time.Nanosecond,
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
}

func TestDNSDiscoverer_Discover_MultipleAddresses(t *testing.T) {
	// Test with a host that typically returns multiple addresses
	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "multi-addr",
		Host:    "google.com",
		Method:  "dns",
		Timeout: 5 * time.Second,
	}

	found, err := d.Discover(ctx, target)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestDNSDiscoverer_Discover_InvalidHostFormat(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "host with port",
			host: "localhost:8080",
		},
		{
			name: "host with spaces",
			host: "local host",
		},
	}

	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := discovery.DiscoveryTarget{
				Name:    tt.name,
				Host:    tt.host,
				Method:  "dns",
				Timeout: 2 * time.Second,
			}

			found, err := d.Discover(ctx, target)
			// Invalid hosts should fail DNS lookup
			assert.False(t, found)
			assert.Error(t, err)
		})
	}
}

func TestDNSDiscoverer_ImplementsDiscoverer(t *testing.T) {
	// Compile-time check that DNSDiscoverer implements Discoverer
	var d discovery.Discoverer = discovery.NewDNSDiscoverer()
	require.NotNil(t, d)
}

// TestDNSDiscoverer_Discover_ZeroTimeout tests that a zero timeout
// uses the default 5 second timeout.
func TestDNSDiscoverer_Discover_ZeroTimeout(t *testing.T) {
	d := discovery.NewDNSDiscoverer()
	ctx := context.Background()

	target := discovery.DiscoveryTarget{
		Name:    "zero-timeout-test",
		Host:    "localhost",
		Method:  "dns",
		Timeout: 0, // Explicitly zero to trigger default
	}

	found, err := d.Discover(ctx, target)
	require.NoError(t, err)
	assert.True(t, found)
}
