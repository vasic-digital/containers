package discovery

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCPDiscoverer implements Discoverer by performing a TCP dial to
// the target host and port.
type TCPDiscoverer struct{}

// NewTCPDiscoverer returns a new TCPDiscoverer.
func NewTCPDiscoverer() *TCPDiscoverer {
	return &TCPDiscoverer{}
}

// Discover attempts a TCP connection to target.Host:target.Port.
// It returns true when the connection succeeds.
func (d *TCPDiscoverer) Discover(
	ctx context.Context,
	target DiscoveryTarget,
) (bool, error) {
	if target.Host == "" || target.Port == "" {
		return false, fmt.Errorf(
			"discovery %s: host and port are required",
			target.Name,
		)
	}

	timeout := target.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	addr := net.JoinHostPort(target.Host, target.Port)

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, fmt.Errorf(
			"discovery %s: tcp dial %s: %w",
			target.Name, addr, err,
		)
	}
	_ = conn.Close()
	return true, nil
}
