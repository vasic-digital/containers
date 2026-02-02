package runtime

import "time"

// ContainerState represents the state of a container.
type ContainerState string

const (
	// StateRunning indicates the container is running.
	StateRunning ContainerState = "running"
	// StateStopped indicates the container is stopped.
	StateStopped ContainerState = "stopped"
	// StateCreated indicates the container has been created but not started.
	StateCreated ContainerState = "created"
	// StatePaused indicates the container is paused.
	StatePaused ContainerState = "paused"
	// StateRestarting indicates the container is restarting.
	StateRestarting ContainerState = "restarting"
	// StateRemoving indicates the container is being removed.
	StateRemoving ContainerState = "removing"
	// StateDead indicates the container is dead.
	StateDead ContainerState = "dead"
)

// ContainerStatus holds current status information for a container.
type ContainerStatus struct {
	ID         string
	Name       string
	State      ContainerState
	Health     string
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
	Ports      []PortMapping
}

// ContainerInfo holds detailed information about a container.
type ContainerInfo struct {
	ID       string
	Name     string
	Image    string
	ImageID  string
	State    ContainerState
	Status   string
	Created  time.Time
	Labels   map[string]string
	Ports    []PortMapping
	Networks []string
}

// ContainerStats holds resource usage statistics for a container.
type ContainerStats struct {
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	NetworkRx     uint64
	NetworkTx     uint64
	BlockRead     uint64
	BlockWrite    uint64
	PIDs          int
}

// PortMapping represents a mapping between host and container ports.
type PortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string
}

// ExecResult holds the output of an executed command in a container.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// ListFilter defines filter criteria for listing containers.
type ListFilter struct {
	All    bool
	Labels map[string]string
	Names  []string
	Status []ContainerState
}
