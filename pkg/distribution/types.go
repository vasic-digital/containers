package distribution

import (
	"time"

	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// DistributionState describes the phase of a distributed
// container.
type DistributionState string

const (
	// StateScheduled means the container has been assigned a host.
	StateScheduled DistributionState = "scheduled"
	// StateDeploying means the container is being deployed.
	StateDeploying DistributionState = "deploying"
	// StateRunning means the container is running on its host.
	StateRunning DistributionState = "running"
	// StateFailed means deployment or health check failed.
	StateFailed DistributionState = "failed"
	// StateMigrating means the container is being moved.
	StateMigrating DistributionState = "migrating"
	// StateStopped means the container has been stopped.
	StateStopped DistributionState = "stopped"
)

// DistributedContainer tracks a single container's placement.
type DistributedContainer struct {
	// Requirement is the original scheduling requirement.
	Requirement scheduler.ContainerRequirements
	// HostName is the host where the container runs.
	HostName string
	// ContainerID is the runtime container ID.
	ContainerID string
	// State is the current distribution state.
	State DistributionState
	// TunnelPorts maps remote ports to local forwarded ports.
	TunnelPorts map[string]string
	// VolumeMounts are the names of mounted volumes.
	VolumeMounts []string
	// DeployedAt is when the container was deployed.
	DeployedAt time.Time
	// Error holds the last error if state is failed.
	Error string
}

// DistributionPlan holds the full plan for a distribution batch.
type DistributionPlan struct {
	// Placement is the scheduler's placement plan.
	Placement *scheduler.PlacementPlan
	// Containers are the tracked distributed containers.
	Containers []DistributedContainer
}

// DistributionSummary reports the outcome of a distribution.
type DistributionSummary struct {
	// TotalContainers is the total number of containers.
	TotalContainers int
	// LocalContainers is the count placed locally.
	LocalContainers int
	// RemoteContainers is the count placed on remote hosts.
	RemoteContainers int
	// FailedContainers is the count that failed.
	FailedContainers int
	// HostUtilization maps host names to their resource snapshots
	// at the time of distribution.
	HostUtilization map[string]*remote.HostResources
	// Duration is how long the distribution took.
	Duration time.Duration
	// Containers lists all distributed containers.
	Containers []DistributedContainer
}
