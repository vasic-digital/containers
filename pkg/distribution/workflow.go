package distribution

// WorkflowPhase describes the phases of a distribution workflow.
type WorkflowPhase string

const (
	// PhaseProbe probes remote hosts for resources.
	PhaseProbe WorkflowPhase = "probe"
	// PhaseSchedule determines container placement.
	PhaseSchedule WorkflowPhase = "schedule"
	// PhaseVolumes mounts remote volumes.
	PhaseVolumes WorkflowPhase = "volumes"
	// PhaseDeploy deploys containers.
	PhaseDeploy WorkflowPhase = "deploy"
	// PhaseTunnels creates SSH tunnels.
	PhaseTunnels WorkflowPhase = "tunnels"
	// PhaseHealth performs health checks.
	PhaseHealth WorkflowPhase = "health"
	// PhaseEvents emits events.
	PhaseEvents WorkflowPhase = "events"
)

// AllPhases returns the 7 phases in execution order.
func AllPhases() []WorkflowPhase {
	return []WorkflowPhase{
		PhaseProbe,
		PhaseSchedule,
		PhaseVolumes,
		PhaseDeploy,
		PhaseTunnels,
		PhaseHealth,
		PhaseEvents,
	}
}

// PhaseDescription returns a human-readable description of the
// workflow phase.
func PhaseDescription(phase WorkflowPhase) string {
	switch phase {
	case PhaseProbe:
		return "Probing remote hosts for resource availability"
	case PhaseSchedule:
		return "Scheduling containers across hosts"
	case PhaseVolumes:
		return "Mounting volumes on remote hosts"
	case PhaseDeploy:
		return "Deploying containers"
	case PhaseTunnels:
		return "Creating SSH tunnels for networking"
	case PhaseHealth:
		return "Running health checks"
	case PhaseEvents:
		return "Emitting distribution events"
	default:
		return "Unknown phase"
	}
}
