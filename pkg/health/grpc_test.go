package health

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckGRPC_Healthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	target := HealthTarget{
		Name:    "test-grpc",
		Host:    "127.0.0.1",
		Port:    port,
		Type:    HealthGRPC,
		Timeout: 2 * time.Second,
	}

	ctx := context.Background()
	result := CheckGRPC(ctx, target)

	assert.True(t, result.Healthy)
	assert.Equal(t, "test-grpc", result.Target)
	assert.Empty(t, result.Error)
	assert.Equal(t, "grpc-tcp", result.Details["protocol"])
}

func TestCheckGRPC_Unhealthy(t *testing.T) {
	target := HealthTarget{
		Name:    "test-grpc-refused",
		Host:    "127.0.0.1",
		Port:    "1",
		Type:    HealthGRPC,
		Timeout: 500 * time.Millisecond,
	}

	ctx := context.Background()
	result := CheckGRPC(ctx, target)

	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "grpc tcp dial failed")
}

func TestCheckGRPC_TableDriven(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, openPort, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tests := []struct {
		name    string
		target  HealthTarget
		healthy bool
		errMsg  string
	}{
		{
			name: "open port succeeds",
			target: HealthTarget{
				Name: "grpc-open", Host: "127.0.0.1",
				Port: openPort, Type: HealthGRPC,
				Timeout: 2 * time.Second,
			},
			healthy: true,
		},
		{
			name: "closed port fails",
			target: HealthTarget{
				Name: "grpc-closed", Host: "127.0.0.1",
				Port: "1", Type: HealthGRPC,
				Timeout: 500 * time.Millisecond,
			},
			healthy: false,
			errMsg:  "grpc tcp dial failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := CheckGRPC(ctx, tt.target)
			assert.Equal(t, tt.healthy, result.Healthy)
			if tt.errMsg != "" {
				assert.Contains(t, result.Error, tt.errMsg)
			}
		})
	}
}

func TestCheckGRPC_RespectsContextCancellation(t *testing.T) {
	target := HealthTarget{
		Name:    "grpc-cancel",
		Host:    "192.0.2.1",
		Port:    "50051",
		Type:    HealthGRPC,
		Timeout: 5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 200*time.Millisecond,
	)
	defer cancel()

	start := time.Now()
	result := CheckGRPC(ctx, target)
	elapsed := time.Since(start)

	assert.False(t, result.Healthy)
	assert.Less(t, elapsed, 2*time.Second)
}
