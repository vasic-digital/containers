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

func TestFailoverHandler_NoHostManagerOrExecutor(t *testing.T) {
	dist := NewDistributor(
		WithLogger(logging.NopLogger{}),
	)
	fh := NewFailoverHandler(dist)

	_, err := fh.CheckAndFailover(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(),
		"host manager and executor required",
	)
}

func TestFailoverHandler_AllHostsOnline(t *testing.T) {
	dist := NewDistributor(
		WithScheduler(&mockScheduler{}),
		WithExecutor(&mockExecutor{}),
		WithHostManager(&mockHostManager{
			hosts: map[string]remote.RemoteHost{
				"remote-1": {
					Name: "remote-1", Address: "10.0.0.1",
					User: "u", Runtime: "docker",
				},
			},
		}),
		WithLogger(logging.NopLogger{}),
	)

	// Distribute some containers.
	reqs := []scheduler.ContainerRequirements{
		{Name: "app-1", Image: "nginx"},
	}
	_, err := dist.Distribute(context.Background(), reqs)
	require.NoError(t, err)

	fh := NewFailoverHandler(dist)
	actions, err := fh.CheckAndFailover(
		context.Background(),
	)
	assert.NoError(t, err)
	assert.Nil(t, actions)
}

func TestFailoverHandler_HostOffline(t *testing.T) {
	dist := NewDistributor(
		WithScheduler(&mockScheduler{
			batchFunc: func(
				ctx context.Context,
				reqs []scheduler.ContainerRequirements,
			) (*scheduler.PlacementPlan, error) {
				decisions := make(
					[]scheduler.PlacementDecision, len(reqs),
				)
				for i, req := range reqs {
					decisions[i] = scheduler.PlacementDecision{
						Requirement: req,
						HostName:    "remote-1",
						Score:       0.7,
						Reason:      "test",
					}
				}
				return &scheduler.PlacementPlan{
					Decisions:     decisions,
					HostSnapshots: map[string]*remote.HostResources{},
				}, nil
			},
		}),
		WithExecutor(&mockExecutor{
			reachableFunc: func(
				ctx context.Context,
				host remote.RemoteHost,
			) bool {
				return false // Host is offline.
			},
		}),
		WithHostManager(&mockHostManager{
			hosts: map[string]remote.RemoteHost{
				"remote-1": {
					Name: "remote-1", Address: "10.0.0.1",
					User: "u", Runtime: "docker",
				},
			},
		}),
		WithLogger(logging.NopLogger{}),
	)

	// Distribute a container to the remote host.
	reqs := []scheduler.ContainerRequirements{
		{Name: "app-1", Image: "nginx"},
	}
	_, err := dist.Distribute(context.Background(), reqs)
	require.NoError(t, err)

	fh := NewFailoverHandler(dist)
	actions, err := fh.CheckAndFailover(
		context.Background(),
	)
	assert.NoError(t, err)
	require.Len(t, actions, 1)
	assert.Equal(t, "app-1", actions[0].ContainerName)
	assert.Equal(t, "remote-1", actions[0].OriginalHost)
	assert.Equal(t, "host offline", actions[0].Reason)
}

func TestFailoverHandler_LocalContainersIgnored(t *testing.T) {
	dist := NewDistributor(
		WithScheduler(&mockScheduler{}),
		WithExecutor(&mockExecutor{}),
		WithHostManager(&mockHostManager{
			hosts: map[string]remote.RemoteHost{},
		}),
		WithLogger(logging.NopLogger{}),
	)

	// Distribute containers locally.
	reqs := []scheduler.ContainerRequirements{
		{Name: "app-1", Image: "nginx"},
	}
	_, err := dist.Distribute(context.Background(), reqs)
	require.NoError(t, err)

	fh := NewFailoverHandler(dist)
	actions, err := fh.CheckAndFailover(
		context.Background(),
	)
	assert.NoError(t, err)
	assert.Nil(t, actions)
}

func TestDetectDegradedHosts(t *testing.T) {
	snapshots := map[string]*remote.HostResources{
		"h1": {
			Host:          "h1",
			CPUPercent:    95.0,
			MemoryPercent: 40.0,
		},
		"h2": {
			Host:          "h2",
			CPUPercent:    30.0,
			MemoryPercent: 92.0,
		},
		"h3": {
			Host:          "h3",
			CPUPercent:    30.0,
			MemoryPercent: 40.0,
		},
	}

	degraded := DetectDegradedHosts(snapshots, 90.0, 90.0)
	assert.Len(t, degraded, 2)
	assert.Contains(t, degraded, "h1")
	assert.Contains(t, degraded, "h2")
}

func TestDetectDegradedHosts_NoneAboveThreshold(t *testing.T) {
	snapshots := map[string]*remote.HostResources{
		"h1": {
			Host: "h1", CPUPercent: 50, MemoryPercent: 60,
		},
	}

	degraded := DetectDegradedHosts(snapshots, 90.0, 90.0)
	assert.Empty(t, degraded)
}

func TestDetectDegradedHosts_EmptySnapshots(t *testing.T) {
	degraded := DetectDegradedHosts(
		map[string]*remote.HostResources{}, 90.0, 90.0,
	)
	assert.Empty(t, degraded)
}

func TestFailoverAction_Fields(t *testing.T) {
	action := FailoverAction{
		ContainerName: "web-1",
		OriginalHost:  "node-a",
		NewHost:       "node-b",
		Reason:        "host offline",
	}
	assert.Equal(t, "web-1", action.ContainerName)
	assert.Equal(t, "node-a", action.OriginalHost)
	assert.Equal(t, "node-b", action.NewHost)
	assert.Equal(t, "host offline", action.Reason)
}
