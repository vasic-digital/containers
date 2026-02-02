package health

import (
	"context"
	"fmt"
	"net"
	"time"
)

const defaultGRPCTimeout = 5 * time.Second

// CheckGRPC performs a gRPC health check. It first verifies TCP
// connectivity to the target address. A full gRPC health/v1 protocol
// check can be layered on top by registering a custom checker; this
// implementation provides a reliable baseline that works without
// importing the gRPC dependency.
func CheckGRPC(ctx context.Context, target HealthTarget) *HealthResult {
	start := time.Now()
	addr := target.ResolvedAddress()

	timeout := target.Timeout
	if timeout <= 0 {
		timeout = defaultGRPCTimeout
	}

	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining < timeout {
			timeout = remaining
		}
	}

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	duration := time.Since(start)

	if err != nil {
		return &HealthResult{
			Target:    target.Name,
			Healthy:   false,
			Duration:  duration,
			Error:     fmt.Sprintf("grpc tcp dial failed: %v", err),
			Timestamp: start,
		}
	}
	_ = conn.Close()

	return &HealthResult{
		Target:    target.Name,
		Healthy:   true,
		Duration:  duration,
		Timestamp: start,
		Details: map[string]string{
			"address":  addr,
			"protocol": "grpc-tcp",
		},
	}
}
