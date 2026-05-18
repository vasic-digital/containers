// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package distribution

import (
	"context"

	"digital.vasic.containers/pkg/i18n"
)

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

// phaseMsgID maps each workflow phase to its CONST-046 message ID
// declared in pkg/i18n/bundles/active.en.yaml. Operators monitoring
// the distribution workflow see the resolved bundle text under any
// non-noop Translator; the noop fallback returns the ID verbatim per
// the §11.9 captured-evidence pattern (no hardcoded English literal
// leaks into wire output regardless of which path is taken).
var phaseMsgID = map[WorkflowPhase]string{
	PhaseProbe:    "containers_workflow_phase_probe",
	PhaseSchedule: "containers_workflow_phase_schedule",
	PhaseVolumes:  "containers_workflow_phase_volumes",
	PhaseDeploy:   "containers_workflow_phase_deploy",
	PhaseTunnels:  "containers_workflow_phase_tunnels",
	PhaseHealth:   "containers_workflow_phase_health",
	PhaseEvents:   "containers_workflow_phase_events",
}

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

// PhaseDescription returns the localised description of the workflow
// phase resolved through the supplied Translator. Per CONST-046 the
// returned text MUST originate from an i18n bundle (or LLM / dynamic
// composition); under the default NoopTranslator the call returns the
// `containers_workflow_phase_*` message ID verbatim — that ID is
// itself positive runtime evidence per CONST-035 / §11.9 (operators
// can map it back to bundles/active.en.yaml without ambiguity).
//
// Pass `nil` for the translator to obtain the noop-fallback behaviour
// (verbatim message-ID return) without constructing a NoopTranslator
// at the call site.
func PhaseDescription(ctx context.Context, translator i18n.Translator, phase WorkflowPhase) string {
	msgID, ok := phaseMsgID[phase]
	if !ok {
		msgID = "containers_workflow_phase_unknown"
	}
	if translator == nil {
		translator = i18n.NoopTranslator{}
	}
	return translator.T(ctx, msgID, nil)
}
