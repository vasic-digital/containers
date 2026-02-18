package scheduler

import (
	"context"
	"fmt"

	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
)

// Scheduler decides where to place containers across available
// hosts.
type Scheduler interface {
	// Schedule determines the best host for a single container.
	Schedule(
		ctx context.Context, req ContainerRequirements,
	) (*PlacementDecision, error)

	// ScheduleBatch determines placement for multiple containers.
	ScheduleBatch(
		ctx context.Context, reqs []ContainerRequirements,
	) (*PlacementPlan, error)

	// Rebalance evaluates current placement and suggests moves.
	Rebalance(ctx context.Context) (*PlacementPlan, error)
}

// DefaultScheduler implements Scheduler using configurable
// strategies.
type DefaultScheduler struct {
	opts        Options
	hostManager remote.HostManager
	scorer      *ResourceScorer
	logger      logging.Logger
	placements  map[string]int // host -> container count
}

// NewScheduler creates a DefaultScheduler.
func NewScheduler(
	hostManager remote.HostManager,
	logger logging.Logger,
	opts ...Option,
) *DefaultScheduler {
	o := ApplyOptions(opts)
	if logger == nil {
		logger = logging.NopLogger{}
	}
	return &DefaultScheduler{
		opts:        o,
		hostManager: hostManager,
		scorer:      NewResourceScorer(o),
		logger:      logger,
		placements:  make(map[string]int),
	}
}

// Schedule determines placement for a single container.
func (s *DefaultScheduler) Schedule(
	ctx context.Context, req ContainerRequirements,
) (*PlacementDecision, error) {
	snapshots := s.hostManager.ProbeAll(ctx)
	hosts := s.hostManager.ListHosts()

	decision := s.scheduleOne(snapshots, hosts, req)
	if decision.HostName != "" {
		s.placements[decision.HostName]++
	}

	s.logger.Info("scheduled %s -> %s (score=%.3f, reason=%s)",
		req.Name, decision.HostName, decision.Score,
		decision.Reason,
	)
	return &decision, nil
}

// ScheduleBatch determines placement for multiple containers.
func (s *DefaultScheduler) ScheduleBatch(
	ctx context.Context, reqs []ContainerRequirements,
) (*PlacementPlan, error) {
	if len(reqs) == 0 {
		return &PlacementPlan{}, nil
	}

	snapshots := s.hostManager.ProbeAll(ctx)
	hosts := s.hostManager.ListHosts()

	plan := &PlacementPlan{
		Decisions:     make([]PlacementDecision, 0, len(reqs)),
		HostSnapshots: snapshots,
	}

	for _, req := range reqs {
		decision := s.scheduleOne(snapshots, hosts, req)
		if decision.HostName != "" {
			s.placements[decision.HostName]++
		}
		plan.Decisions = append(plan.Decisions, decision)

		s.logger.Info(
			"batch: scheduled %s -> %s (score=%.3f)",
			req.Name, decision.HostName, decision.Score,
		)
	}

	return plan, nil
}

// Rebalance suggests redistributing existing containers.
func (s *DefaultScheduler) Rebalance(
	ctx context.Context,
) (*PlacementPlan, error) {
	snapshots := s.hostManager.ProbeAll(ctx)
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no host snapshots available")
	}

	plan := &PlacementPlan{
		HostSnapshots: snapshots,
	}

	// Identify overloaded hosts (>80% CPU or memory).
	for name, snap := range snapshots {
		if snap.CPUPercent > 80 || snap.MemoryPercent > 80 {
			s.logger.Warn(
				"host %s overloaded: CPU=%.1f%% Mem=%.1f%%",
				name, snap.CPUPercent, snap.MemoryPercent,
			)
		}
	}

	return plan, nil
}

func (s *DefaultScheduler) scheduleOne(
	snapshots map[string]*remote.HostResources,
	hosts []remote.RemoteHost,
	req ContainerRequirements,
) PlacementDecision {
	switch s.opts.Strategy {
	case StrategyRoundRobin:
		return scheduleRoundRobin(
			hosts, req, s.opts.LocalHostName,
		)
	case StrategyAffinity:
		return scheduleAffinity(
			s.scorer, snapshots, hosts, req,
		)
	case StrategySpread:
		return scheduleSpread(
			snapshots, hosts, req,
			s.opts.LocalHostName, s.placements,
		)
	case StrategyBinPack:
		return scheduleBinPack(
			s.scorer, snapshots, hosts, req,
			s.opts.LocalHostName,
		)
	default:
		return scheduleResourceAware(
			s.scorer, snapshots, hosts, req,
			s.opts.LocalHostName,
		)
	}
}
