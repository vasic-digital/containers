package network

import "time"

// TunnelDirection indicates the direction of an SSH tunnel.
type TunnelDirection string

const (
	// TunnelLocal forwards a remote port to a local port
	// (ssh -L). Remote service becomes accessible locally.
	TunnelLocal TunnelDirection = "local"
	// TunnelRemote forwards a local port to a remote port
	// (ssh -R). Local service becomes accessible remotely.
	TunnelRemote TunnelDirection = "remote"
)

// TunnelState describes the current state of a tunnel.
type TunnelState string

const (
	// TunnelActive means the tunnel is running.
	TunnelActive TunnelState = "active"
	// TunnelClosed means the tunnel has been closed.
	TunnelClosed TunnelState = "closed"
	// TunnelFailed means the tunnel failed to establish.
	TunnelFailed TunnelState = "failed"
	// TunnelReconnecting means the tunnel is being re-established.
	TunnelReconnecting TunnelState = "reconnecting"
)

// TunnelSpec describes a tunnel to create.
type TunnelSpec struct {
	// Direction is local or remote forwarding.
	Direction TunnelDirection
	// LocalPort is the local end of the tunnel.
	LocalPort string
	// RemoteHost is the target host for the forwarded connection.
	RemoteHost string
	// RemotePort is the target port for the forwarded connection.
	RemotePort string
	// Description is a human-readable label.
	Description string
}

// TunnelInfo holds the state of an active tunnel.
type TunnelInfo struct {
	// Spec is the original tunnel specification.
	Spec TunnelSpec
	// HostName is the SSH host this tunnel goes through.
	HostName string
	// State is the current tunnel state.
	State TunnelState
	// CreatedAt is when the tunnel was created.
	CreatedAt time.Time
	// PID is the SSH process ID managing this tunnel.
	PID int
}

// PortAllocation tracks a port assignment.
type PortAllocation struct {
	// Port is the allocated port number.
	Port int
	// Description labels what the port is used for.
	Description string
	// AllocatedAt is when the port was allocated.
	AllocatedAt time.Time
}
