package buildpkg

import (
	"context"
	"fmt"

	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

type Planner struct {
	hostManager remote.HostManager
	scheduler   scheduler.Scheduler
}

func NewPlanner(hostManager remote.HostManager) *Planner {
	sched := scheduler.NewScheduler(
		hostManager, nil,
		scheduler.WithStrategy(scheduler.StrategyResourceAware),
	)
	return &Planner{hostManager: hostManager, scheduler: sched}
}

func NewPlannerWithScheduler(hostManager remote.HostManager, sched scheduler.Scheduler) *Planner {
	return &Planner{hostManager: hostManager, scheduler: sched}
}

func (p *Planner) PlanAll(ctx context.Context) (*BuildPlan, error) {
	components := AllComponents()
	return p.plan(ctx, components)
}

func (p *Planner) PlanSingle(ctx context.Context, componentName string) (*BuildPlan, error) {
	component, err := FindComponent(componentName)
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
