package monitor

import (
	"context"
	"fmt"

	"digital.vasic.containers/pkg/runtime"
)

// ContainerCollector gathers resource metrics for individual
// containers using the container runtime.
type ContainerCollector struct {
	rt runtime.ContainerRuntime
}

// NewContainerCollector creates a ContainerCollector backed by the
// given container runtime.
func NewContainerCollector(
	rt runtime.ContainerRuntime,
) *ContainerCollector {
	return &ContainerCollector{rt: rt}
}

// CollectAll queries the runtime for all running containers and
// returns their resource metrics keyed by container name.
func (c *ContainerCollector) CollectAll(
	ctx context.Context,
) (map[string]ContainerResources, error) {
	containers, err := c.rt.List(ctx, runtime.ListFilter{
		All:    false,
		Status: []runtime.ContainerState{runtime.StateRunning},
	})
	if err != nil {
		return nil, fmt.Errorf(
			"container collector: list: %w", err,
		)
	}

	result := make(map[string]ContainerResources, len(containers))
	for _, info := range containers {
		stats, sErr := c.rt.Stats(ctx, info.ID)
		if sErr != nil {
			continue
		}
		result[info.Name] = ContainerResources{
			Name:          info.Name,
			CPUPercent:    stats.CPUPercent,
			MemoryPercent: stats.MemoryPercent,
			MemoryUsage:   stats.MemoryUsage,
			MemoryLimit:   stats.MemoryLimit,
		}
	}
	return result, nil
}

// Collect queries the runtime for a single container's resource
// metrics.
func (c *ContainerCollector) Collect(
	ctx context.Context,
	containerID string,
) (*ContainerResources, error) {
	stats, err := c.rt.Stats(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf(
			"container collector: stats %s: %w",
			containerID, err,
		)
	}

	status, err := c.rt.Status(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf(
			"container collector: status %s: %w",
			containerID, err,
		)
	}

	return &ContainerResources{
		Name:          status.Name,
		CPUPercent:    stats.CPUPercent,
		MemoryPercent: stats.MemoryPercent,
		MemoryUsage:   stats.MemoryUsage,
		MemoryLimit:   stats.MemoryLimit,
	}, nil
}
