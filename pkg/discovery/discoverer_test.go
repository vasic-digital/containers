package discovery_test

import (
	"context"
	"testing"
	"time"

	"digital.vasic.containers/pkg/discovery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDiscoverer is a test double that returns preset values.
type mockDiscoverer struct {
	found bool
	err   error
}

func (m *mockDiscoverer) Discover(
	_ context.Context,
	_ discovery.DiscoveryTarget,
) (bool, error) {
	return m.found, m.err
}

func TestDiscoverer_Interface(t *testing.T) {
	var d discovery.Discoverer = &mockDiscoverer{found: true}
	ctx := context.Background()
	target := discovery.DiscoveryTarget{
		Name:    "test-svc",
		Host:    "localhost",
		Port:    "8080",
		Method:  "tcp",
		Timeout: time.Second,
	}

	found, err := d.Discover(ctx, target)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestDiscoveryTarget_Fields(t *testing.T) {
	target := discovery.DiscoveryTarget{
		Name:    "redis",
		Host:    "redis.local",
		Port:    "6379",
		Method:  "tcp",
		Timeout: 3 * time.Second,
	}

	assert.Equal(t, "redis", target.Name)
	assert.Equal(t, "redis.local", target.Host)
	assert.Equal(t, "6379", target.Port)
	assert.Equal(t, "tcp", target.Method)
	assert.Equal(t, 3*time.Second, target.Timeout)
}
