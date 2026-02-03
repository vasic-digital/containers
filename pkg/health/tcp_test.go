package health

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTCP_Healthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	target := HealthTarget{
		Name:    "test-tcp",
		Host:    "127.0.0.1",
		Port:    port,
		Type:    HealthTCP,
		Timeout: 2 * time.Second,
	}

	ctx := context.Background()
	result := CheckTCP(ctx, target)

	assert.True(t, result.Healthy)
	assert.Equal(t, "test-tcp", result.Target)
	assert.Empty(t, result.Error)
	assert.NotZero(t, result.Duration)
	assert.Contains(t, result.Details["address"], port)
}

func TestCheckTCP_Unhealthy_RefusedConnection(t *testing.T) {
	// Use a port that is very unlikely to be open.
	target := HealthTarget{
		Name:    "test-tcp-refused",
		Host:    "127.0.0.1",
		Port:    "1",
		Type:    HealthTCP,
		Timeout: 500 * time.Millisecond,
	}

	ctx := context.Background()
	result := CheckTCP(ctx, target)

	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "tcp dial failed")
}

func TestCheckTCP_TableDriven(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, openPort, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tests := []struct {
		name    string
		target  HealthTarget
		healthy bool
	}{
		{
			name: "open port succeeds",
			target: HealthTarget{
				Name: "open", Host: "127.0.0.1", Port: openPort,
				Type: HealthTCP, Timeout: 2 * time.Second,
			},
			healthy: true,
		},
		{
			name: "closed port fails",
			target: HealthTarget{
				Name: "closed", Host: "127.0.0.1", Port: "1",
				Type: HealthTCP, Timeout: 500 * time.Millisecond,
			},
			healthy: false,
		},
		{
			name: "empty address fails",
			target: HealthTarget{
				Name: "empty", Host: "", Port: "",
				Type: HealthTCP, Timeout: 500 * time.Millisecond,
			},
			healthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := CheckTCP(ctx, tt.target)
			assert.Equal(t, tt.healthy, result.Healthy)
		})
	}
}

func TestCheckTCP_RespectsContextDeadline(t *testing.T) {
	// Use an address that will hang (non-routable IP).
	target := HealthTarget{
		Name:    "timeout-test",
		Host:    "192.0.2.1",
		Port:    "12345",
		Type:    HealthTCP,
		Timeout: 5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 200*time.Millisecond,
	)
	defer cancel()

	start := time.Now()
	result := CheckTCP(ctx, target)
	elapsed := time.Since(start)

	assert.False(t, result.Healthy)
	// Should complete in roughly the context timeout, not 5s.
	assert.Less(t, elapsed, 2*time.Second)
}

func TestCheckTCP_DefaultTimeout(t *testing.T) {
	// Use a non-routable IP to trigger timeout.
	target := HealthTarget{
		Name:    "default-timeout",
		Host:    "192.0.2.1",
		Port:    "12345",
		Type:    HealthTCP,
		Timeout: 0, // Should use defaultTCPTimeout (5s).
	}

	// Set context deadline shorter than default to verify code path.
	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	result := CheckTCP(ctx, target)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "tcp dial failed")
}

func TestCheckTCP_NegativeTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	target := HealthTarget{
		Name:    "negative-timeout",
		Host:    "127.0.0.1",
		Port:    port,
		Type:    HealthTCP,
		Timeout: -1 * time.Second, // Negative should trigger default.
	}

	ctx := context.Background()
	result := CheckTCP(ctx, target)

	assert.True(t, result.Healthy)
}
