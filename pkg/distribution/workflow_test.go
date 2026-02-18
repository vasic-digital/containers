package distribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllPhases(t *testing.T) {
	phases := AllPhases()
	assert.Len(t, phases, 7)
	assert.Equal(t, PhaseProbe, phases[0])
	assert.Equal(t, PhaseSchedule, phases[1])
	assert.Equal(t, PhaseVolumes, phases[2])
	assert.Equal(t, PhaseDeploy, phases[3])
	assert.Equal(t, PhaseTunnels, phases[4])
	assert.Equal(t, PhaseHealth, phases[5])
	assert.Equal(t, PhaseEvents, phases[6])
}

func TestPhaseDescription(t *testing.T) {
	tests := []struct {
		phase WorkflowPhase
		want  string
	}{
		{PhaseProbe, "Probing remote hosts for resource availability"},
		{PhaseSchedule, "Scheduling containers across hosts"},
		{PhaseVolumes, "Mounting volumes on remote hosts"},
		{PhaseDeploy, "Deploying containers"},
		{PhaseTunnels, "Creating SSH tunnels for networking"},
		{PhaseHealth, "Running health checks"},
		{PhaseEvents, "Emitting distribution events"},
		{WorkflowPhase("invalid"), "Unknown phase"},
	}
	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			got := PhaseDescription(tt.phase)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorkflowPhase_String(t *testing.T) {
	assert.Equal(t, "probe", string(PhaseProbe))
	assert.Equal(t, "schedule", string(PhaseSchedule))
	assert.Equal(t, "volumes", string(PhaseVolumes))
	assert.Equal(t, "deploy", string(PhaseDeploy))
	assert.Equal(t, "tunnels", string(PhaseTunnels))
	assert.Equal(t, "health", string(PhaseHealth))
	assert.Equal(t, "events", string(PhaseEvents))
}
