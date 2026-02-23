//go:build !integration

package distribution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// TestDistribute_ScoreZero verifies that when a scheduler returns a
// PlacementDecision with Score=0, Distribute marks that container as
// StateFailed and increments FailedContainers.
func TestDistribute_ScoreZero(t *testing.T) {
	// Scheduler that returns Score=0 for every decision.
	zeroScoreScheduler := &mockScheduler{
		batchFunc: func(
			ctx context.Context,
			reqs []scheduler.ContainerRequirements,
		) (*scheduler.PlacementPlan, error) {
			decisions := make([]scheduler.PlacementDecision, len(reqs))
			for i, req := range reqs {
				decisions[i] = scheduler.PlacementDecision{
					Requirement: req,
					HostName:    "local",
					Score:       0, // zero score triggers failed branch
					Reason:      "no capacity",
				}
			}
			return &scheduler.PlacementPlan{
				Decisions:     decisions,
				HostSnapshots: map[string]*remote.HostResources{},
			}, nil
		},
	}

	dist := NewDistributor(
		WithScheduler(zeroScoreScheduler),
		WithLogger(logging.NopLogger{}),
	)

	reqs := []scheduler.ContainerRequirements{
		{Name: "app-1", Image: "nginx"},
		{Name: "app-2", Image: "redis"},
	}

	summary, err := dist.Distribute(context.Background(), reqs)
	require.NoError(t, err)

	assert.Equal(t, 2, summary.TotalContainers)
	assert.Equal(t, 2, summary.FailedContainers)
	assert.Equal(t, 0, summary.LocalContainers)
	assert.Equal(t, 0, summary.RemoteContainers)

	// All containers should be in StateFailed state.
	for _, dc := range summary.Containers {
		assert.Equal(t, StateFailed, dc.State,
			"container %s should be failed", dc.Requirement.Name)
		assert.Equal(t, "no capacity", dc.Error)
	}
}

// TestCheckAndFailover_WithOfflineHost verifies that CheckAndFailover
// detects containers on an unreachable host and collects them for
// rescheduling.
func TestCheckAndFailover_WithOfflineHost(t *testing.T) {
	offlineHost := remote.RemoteHost{
		Name:    "node-offline",
		Address: "192.168.1.50",
		User:    "admin",
		Runtime: "docker",
	}

	hm := &mockHostManager{
		hosts: map[string]remote.RemoteHost{
			"node-offline": offlineHost,
		},
	}

	// Executor that always reports the host as unreachable.
	exec := &mockExecutor{
		reachableFunc: func(
			ctx context.Context,
			host remote.RemoteHost,
		) bool {
			return false // offline
		},
	}

	// Scheduler that reschedules to a different host.
	reschedScheduler := &mockScheduler{
		batchFunc: func(
			ctx context.Context,
			reqs []scheduler.ContainerRequirements,
		) (*scheduler.PlacementPlan, error) {
			decisions := make([]scheduler.PlacementDecision, len(reqs))
			for i, req := range reqs {
				decisions[i] = scheduler.PlacementDecision{
					Requirement: req,
					HostName:    "node-backup",
					Score:       0.9,
					Reason:      "failover",
				}
			}
			return &scheduler.PlacementPlan{
				Decisions:     decisions,
				HostSnapshots: map[string]*remote.HostResources{},
			}, nil
		},
	}

	dist := NewDistributor(
		WithScheduler(reschedScheduler),
		WithExecutor(exec),
		WithHostManager(hm),
		WithLogger(logging.NopLogger{}),
	)

	// Manually populate containers as if they were previously deployed.
	dist.containers = []DistributedContainer{
		{
			Requirement: scheduler.ContainerRequirements{
				Name:  "web-1",
				Image: "nginx",
			},
			HostName: "node-offline",
			State:    StateRunning,
		},
		{
			Requirement: scheduler.ContainerRequirements{
				Name:  "db-1",
				Image: "postgres",
			},
			HostName: "node-offline",
			State:    StateRunning,
		},
	}

	fh := NewFailoverHandler(dist)
	actions, err := fh.CheckAndFailover(context.Background())
	require.NoError(t, err)

	// Both containers should have failover actions.
	require.Len(t, actions, 2)

	names := make(map[string]bool)
	for _, a := range actions {
		names[a.ContainerName] = true
		assert.Equal(t, "node-offline", a.OriginalHost)
		assert.Equal(t, "host offline", a.Reason)
		assert.Equal(t, "node-backup", a.NewHost)
	}
	assert.True(t, names["web-1"])
	assert.True(t, names["db-1"])
}
