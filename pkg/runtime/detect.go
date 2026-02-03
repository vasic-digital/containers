package runtime

import (
	"context"
	"fmt"
)

// RuntimeFactory creates container runtimes. This allows dependency injection
// for testing.
type RuntimeFactory func() []ContainerRuntime

// defaultRuntimeFactory creates the standard set of container runtimes.
func defaultRuntimeFactory() []ContainerRuntime {
	return []ContainerRuntime{
		NewDockerRuntime(),
		NewPodmanRuntime(),
		NewKubernetesRuntime(),
	}
}

// autoDetectWith performs auto-detection using the provided runtimes.
func autoDetectWith(
	ctx context.Context,
	runtimes []ContainerRuntime,
) (ContainerRuntime, error) {
	for _, rt := range runtimes {
		if rt.IsAvailable(ctx) {
			return rt, nil
		}
	}
	return nil, fmt.Errorf(
		"no container runtime detected: " +
			"tried docker, podman, kubernetes",
	)
}

// detectAllWith returns all available runtimes from the provided list.
func detectAllWith(
	ctx context.Context,
	runtimes []ContainerRuntime,
) []ContainerRuntime {
	var available []ContainerRuntime
	for _, rt := range runtimes {
		if rt.IsAvailable(ctx) {
			available = append(available, rt)
		}
	}
	return available
}

// AutoDetect tries Docker first, then Podman, then Kubernetes,
// and returns the first available container runtime.
func AutoDetect(ctx context.Context) (ContainerRuntime, error) {
	return autoDetectWith(ctx, defaultRuntimeFactory())
}

// DetectAll returns all available container runtimes on the system.
func DetectAll(ctx context.Context) []ContainerRuntime {
	return detectAllWith(ctx, defaultRuntimeFactory())
}
