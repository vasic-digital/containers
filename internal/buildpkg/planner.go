package buildpkg

import (
	"context"
	"fmt"

	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// Planner assigns BuildComponents to hosts via the configured
// scheduler. The component catalogue is caller-supplied so this
// package stays project-agnostic — every project passes its own
// []BuildComponent into NewPlanner.
type Planner struct {
	hostManager remote.HostManager
	scheduler   scheduler.Scheduler
	components  []BuildComponent
}

// NewPlanner builds a Planner with the default resource-aware
// scheduler. The components slice is the full list of buildable
// components in the project; an empty slice is valid but makes
// PlanAll a no-op.
func NewPlanner(hostManager remote.HostManager, components []BuildComponent) *Planner {
	sched := scheduler.NewScheduler(
		hostManager, nil,
		scheduler.WithStrategy(scheduler.StrategyResourceAware),
	)
	return &Planner{
		hostManager: hostManager,
		scheduler:   sched,
		components:  append([]BuildComponent(nil), components...),
	}
}

// NewPlannerWithScheduler allows callers to supply a custom scheduler
// alongside the component catalogue.
func NewPlannerWithScheduler(hostManager remote.HostManager, sched scheduler.Scheduler, components []BuildComponent) *Planner {
	return &Planner{
		hostManager: hostManager,
		scheduler:   sched,
		components:  append([]BuildComponent(nil), components...),
	}
}

// Components returns a copy of the configured component catalogue.
func (p *Planner) Components() []BuildComponent {
	out := make([]BuildComponent, len(p.components))
	copy(out, p.components)
	return out
}

// PlanAll plans a build across every component the caller registered.
func (p *Planner) PlanAll(ctx context.Context) (*BuildPlan, error) {
	return p.plan(ctx, p.components)
}

// PlanSingle plans a build for the single named component. The lookup
// uses the caller-supplied component catalogue.
func (p *Planner) PlanSingle(ctx context.Context, componentName string) (*BuildPlan, error) {
	component, err := FindComponentIn(p.components, componentName)
	if err != nil {
		return nil, fmt.Errorf("plan single: %w", err)
	}
	return p.plan(ctx, []BuildComponent{component})
}

func (p *Planner) plan(ctx context.Context, components []BuildComponent) (*BuildPlan, error) {
	reqs := make([]scheduler.ContainerRequirements, len(components))
	for i, c := range components {
		reqs[i] = c.ResourceRequirements()
	}

	placementPlan, err := p.scheduler.ScheduleBatch(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("schedule batch: %w", err)
	}

	assignments := make([]BuildAssignment, len(placementPlan.Decisions))
	for i, decision := range placementPlan.Decisions {
		host := decision.HostName
		if host == "" {
			host = "local"
		}
		assignments[i] = BuildAssignment{
			Component: components[i],
			Host:      host,
		}
	}

	return &BuildPlan{Assignments: assignments}, nil
}
