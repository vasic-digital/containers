package distribution

import (
	"context"
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

// TestPhaseDescription asserts that the i18n-aware PhaseDescription
// returns the namespaced message ID under the NoopTranslator default
// (translator=nil). The verbatim message-ID return is itself positive
// runtime evidence per CONST-035 / §11.9 — operators can map each ID
// back to pkg/i18n/bundles/active.en.yaml without ambiguity.
func TestPhaseDescription(t *testing.T) {
	tests := []struct {
		phase WorkflowPhase
		want  string
	}{
		{PhaseProbe, "containers_workflow_phase_probe"},
		{PhaseSchedule, "containers_workflow_phase_schedule"},
		{PhaseVolumes, "containers_workflow_phase_volumes"},
		{PhaseDeploy, "containers_workflow_phase_deploy"},
		{PhaseTunnels, "containers_workflow_phase_tunnels"},
		{PhaseHealth, "containers_workflow_phase_health"},
		{PhaseEvents, "containers_workflow_phase_events"},
		{WorkflowPhase("invalid"), "containers_workflow_phase_unknown"},
	}
	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			got := PhaseDescription(context.Background(), nil, tt.phase)
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
