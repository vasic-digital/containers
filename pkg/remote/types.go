package remote

import "time"

// AuthMethod identifies the type of authentication used to connect
// to a remote host.
type AuthMethod string

const (
	// AuthSSHKey authenticates using an SSH private key.
	AuthSSHKey AuthMethod = "ssh_key"
	// AuthSSHAgent authenticates via ssh-agent.
	AuthSSHAgent AuthMethod = "ssh_agent"
	// AuthPassword authenticates using a password.
	AuthPassword AuthMethod = "password"
)

// HostState describes the current reachability of a remote host.
type HostState string

const (
	// HostOnline means the host is reachable and responsive.
	HostOnline HostState = "online"
	// HostOffline means the host is unreachable.
	HostOffline HostState = "offline"
	// HostDegraded means the host is reachable but resource-constrained.
	HostDegraded HostState = "degraded"
	// HostUnknown means the host has not been probed yet.
	HostUnknown HostState = "unknown"
)

// RemoteHost describes a remote machine that can run containers.
type RemoteHost struct {
	// Name is a unique identifier for this host.
	Name string
	// Address is the hostname or IP address.
	Address string
	// Port is the SSH port (default 22).
	Port int
	// User is the SSH user.
	User string
	// KeyPath is the path to the SSH private key.
	KeyPath string
	// Password is used when AuthMethod is AuthPassword.
	Password string
	// Auth is the authentication method.
	Auth AuthMethod
	// Runtime is the container runtime on this host
	// (e.g., "docker", "podman").
	Runtime string
	// Labels are arbitrary key-value metadata for scheduling.
	Labels map[string]string
	// MaxContainers limits how many containers this host can run.
	// Zero means no limit.
	MaxContainers int
}

// SSHPort returns the configured SSH port, defaulting to 22.
func (h RemoteHost) SSHPort() int {
	if h.Port <= 0 {
		return 22
	}
	return h.Port
}

// HostResources captures a point-in-time snapshot of host resources.
type HostResources struct {
	// Host is the name of the host these resources belong to.
	Host string
	// Timestamp is when the snapshot was taken.
	Timestamp time.Time
	// CPUPercent is the current CPU usage (0-100).
	CPUPercent float64
	// MemoryPercent is the current memory usage (0-100).
	MemoryPercent float64
	// MemoryTotalMB is the total memory in megabytes.
	MemoryTotalMB uint64
	// MemoryUsedMB is the used memory in megabytes.
	MemoryUsedMB uint64
	// DiskPercent is the current disk usage (0-100).
	DiskPercent float64
	// DiskTotalMB is the total disk in megabytes.
	DiskTotalMB uint64
	// DiskUsedMB is the used disk in megabytes.
	DiskUsedMB uint64
	// LoadAvg1 is the 1-minute load average.
	LoadAvg1 float64
	// LoadAvg5 is the 5-minute load average.
	LoadAvg5 float64
	// LoadAvg15 is the 15-minute load average.
	LoadAvg15 float64
	// CPUCores is the number of CPU cores.
	CPUCores int
	// RunningContainers is the number of running containers.
	RunningContainers int
	// NetworkRxBytesPerSec is the network receive rate.
	NetworkRxBytesPerSec uint64
	// NetworkTxBytesPerSec is the network transmit rate.
	NetworkTxBytesPerSec uint64
	// GPU is the list of GPU devices on this host; nil if none.
	GPU []GPUDevice `json:"gpu,omitempty"`
}

// AvailableMemoryPercent returns the percentage of free memory.
func (r *HostResources) AvailableMemoryPercent() float64 {
	return 100.0 - r.MemoryPercent
}

// AvailableDiskPercent returns the percentage of free disk space.
func (r *HostResources) AvailableDiskPercent() float64 {
	return 100.0 - r.DiskPercent
}

// AvailableCPUPercent returns the percentage of free CPU.
func (r *HostResources) AvailableCPUPercent() float64 {
	return 100.0 - r.CPUPercent
}

// CommandResult holds the output of a command executed on a remote
// host.
type CommandResult struct {
	// Stdout is the standard output.
	Stdout string
	// Stderr is the standard error output.
	Stderr string
	// ExitCode is the process exit code.
	ExitCode int
	// Duration is how long the command took.
	Duration time.Duration
}
