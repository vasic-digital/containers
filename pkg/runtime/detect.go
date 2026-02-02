package runtime

import (
	"context"
	"fmt"
)

// AutoDetect tries Docker first, then Podman, then Kubernetes,
// and returns the first available container runtime.
func AutoDetect(ctx context.Context) (ContainerRuntime, error) {
	docker := NewDockerRuntime()
	if docker.IsAvailable(ctx) {
		return docker, nil
	}

	podman := NewPodmanRuntime()
	if podman.IsAvailable(ctx) {
		return podman, nil
	}

	kube := NewKubernetesRuntime()
	if kube.IsAvailable(ctx) {
		return kube, nil
	}

	return nil, fmt.Errorf(
		"no container runtime detected: " +
			"tried docker, podman, kubernetes",
	)
}

// DetectAll returns all available container runtimes on the system.
func DetectAll(ctx context.Context) []ContainerRuntime {
	var runtimes []ContainerRuntime

	docker := NewDockerRuntime()
	if docker.IsAvailable(ctx) {
		runtimes = append(runtimes, docker)
	}

	podman := NewPodmanRuntime()
	if podman.IsAvailable(ctx) {
		runtimes = append(runtimes, podman)
	}

	kube := NewKubernetesRuntime()
	if kube.IsAvailable(ctx) {
		runtimes = append(runtimes, kube)
	}

	return runtimes
}
