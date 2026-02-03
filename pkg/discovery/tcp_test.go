package discovery_test

import (
	"context"
	"net"
	"testing"
	"time"

	"digital.vasic.containers/pkg/discovery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPDiscoverer_Discover_Success(t *testing.T) {
	// Start a local TCP listener to simulate a reachable service.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	d := discovery.NewTCPDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "local-svc",
		Host:    "127.0.0.1",
		Port:    port,
		Method:  "tcp",
		Timeout: 2 * time.Second,
	}

	found, discErr := d.Discover(ctx, target)
	require.NoError(t, discErr)
	assert.True(t, found)
}

func TestTCPDiscoverer_Discover_Unreachable(t *testing.T) {
	d := discovery.NewTCPDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "absent-svc",
		Host:    "127.0.0.1",
		Port:    "1", // unlikely to be listening
		Method:  "tcp",
		Timeout: 500 * time.Millisecond,
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
}

func TestTCPDiscoverer_Discover_MissingHost(t *testing.T) {
	d := discovery.NewTCPDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name: "no-host",
		Port: "8080",
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host and port are required")
}

func TestTCPDiscoverer_Discover_MissingPort(t *testing.T) {
	d := discovery.NewTCPDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name: "no-port",
		Host: "127.0.0.1",
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host and port are required")
}

func TestTCPDiscoverer_Discover_CancelledContext(t *testing.T) {
	d := discovery.NewTCPDiscoverer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	target := discovery.DiscoveryTarget{
		Name:    "cancelled",
		Host:    "127.0.0.1",
		Port:    "9999",
		Timeout: 5 * time.Second,
	}

	found, err := d.Discover(ctx, target)
	assert.False(t, found)
	assert.Error(t, err)
}

// TestTCPDiscoverer_Discover_ZeroTimeout tests that a zero timeout
// uses the default 5 second timeout.
func TestTCPDiscoverer_Discover_ZeroTimeout(t *testing.T) {
	// Start a local TCP listener to simulate a reachable service.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	d := discovery.NewTCPDiscoverer()
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "zero-timeout-svc",
		Host:    "127.0.0.1",
		Port:    port,
		Method:  "tcp",
		Timeout: 0, // Explicitly zero to trigger default
	}

	found, discErr := d.Discover(ctx, target)
	require.NoError(t, discErr)
	assert.True(t, found)
}
