package scheduler

import "digital.vasic.containers/pkg/remote"

// PlacementStrategy determines how containers are distributed
// across available hosts.
type PlacementStrategy string

const (
	// StrategyResourceAware places containers on the host with
	// the most available resources.
	StrategyResourceAware PlacementStrategy = "resource_aware"
	// StrategyRoundRobin distributes containers evenly across
	// hosts in rotation.
	StrategyRoundRobin PlacementStrategy = "round_robin"
	// StrategyAffinity places containers on hosts matching
	// required labels.
	StrategyAffinity PlacementStrategy = "affinity"
	// StrategySpread distributes containers to minimize
	// per-host density.
	StrategySpread PlacementStrategy = "spread"
	// StrategyBinPack packs containers tightly onto as few hosts
	// as possible.
	StrategyBinPack PlacementStrategy = "bin_pack"
)

// ContainerRequirements describes what a container needs from
// its host.
type ContainerRequirements struct {
	// Name is a human-readable name for the container.
	Name string
	// Image is the container image to run.
	Image string
	// CPUCores is the minimum CPU cores required.
	CPUCores float64
	// MemoryMB is the minimum memory in megabytes.
	MemoryMB uint64
	// DiskMB is the minimum disk space in megabytes.
	DiskMB uint64
	// Labels are required host labels (key=value). The host
	// must have all labels to be eligible.
	Labels map[string]string
	// PreferLocal indicates a preference for local execution.
	PreferLocal bool
	// ComposeFile is the compose file if this is a compose service.
	ComposeFile string
	// ServiceName is the service name within the compose file.
	ServiceName string
}

// PlacementDecision records where a single container was placed.
type PlacementDecision struct {
	// Requirement is the original requirement.
	Requirement ContainerRequirements
	// HostName is the selected host (empty string = local).
	HostName string
	// Score is the placement score (0.0-1.0).
	Score float64
	// Reason explains why this host was chosen.
	Reason string
}

// IsLocal returns true if the container was placed on localhost.
func (d *PlacementDecision) IsLocal() bool {
	return d.HostName == "" || d.HostName == "local"
}

// PlacementPlan holds decisions for a batch of containers.
type PlacementPlan struct {
	// Decisions is the list of placement decisions.
	Decisions []PlacementDecision
	// HostSnapshots is the resource state used for scheduling.
	HostSnapshots map[string]*remote.HostResources
}

// LocalDecisions returns only the decisions placed locally.
func (p *PlacementPlan) LocalDecisions() []PlacementDecision {
	var local []PlacementDecision
	for _, d := range p.Decisions {
		if d.IsLocal() {
			local = append(local, d)
		}
	}
	return local
}

// RemoteDecisions returns only the decisions placed on remote
// hosts.
func (p *PlacementPlan) RemoteDecisions() []PlacementDecision {
	var remote []PlacementDecision
	for _, d := range p.Decisions {
		if !d.IsLocal() {
			remote = append(remote, d)
		}
	}
	return remote
}

// ByHost groups decisions by host name.
func (p *PlacementPlan) ByHost() map[string][]PlacementDecision {
	groups := make(map[string][]PlacementDecision)
	for _, d := range p.Decisions {
		groups[d.HostName] = append(groups[d.HostName], d)
	}
	return groups
}
