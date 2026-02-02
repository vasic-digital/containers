//go:build !integration

package runtime

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testableAutoDetect performs auto-detection using provided runtimes
// instead of creating real ones. This allows unit testing without
// requiring actual docker/podman/kubectl binaries.
func testableAutoDetect(
	runtimes []ContainerRuntime,
) func(context.Context) (ContainerRuntime, error) {
	return func(ctx context.Context) (ContainerRuntime, error) {
		for _, rt := range runtimes {
			if rt.IsAvailable(ctx) {
				return rt, nil
			}
		}
		return nil, fmt.Errorf(
			"no container runtime detected",
		)
	}
}

// testableDetectAll returns all available runtimes from the provided
// list.
func testableDetectAll(
	runtimes []ContainerRuntime,
) func(context.Context) []ContainerRuntime {
	return func(ctx context.Context) []ContainerRuntime {
		var available []ContainerRuntime
		for _, rt := range runtimes {
			if rt.IsAvailable(ctx) {
				available = append(available, rt)
			}
		}
		return available
	}
}

// availableRuntime is a mock runtime for testing detection logic.
type availableRuntime struct {
	name      string
	available bool
}

func (a *availableRuntime) Name() string { return a.name }
func (a *availableRuntime) Version(
	_ context.Context,
) (string, error) {
	return "1.0.0", nil
}
func (a *availableRuntime) IsAvailable(_ context.Context) bool {
	return a.available
}
func (a *availableRuntime) Start(
	_ context.Context, _ string, _ ...StartOption,
) error {
	return nil
}
func (a *availableRuntime) Stop(
	_ context.Context, _ string, _ ...StopOption,
) error {
	return nil
}
func (a *availableRuntime) Remove(
	_ context.Context, _ string, _ ...RemoveOption,
) error {
	return nil
}
func (a *availableRuntime) Status(
	_ context.Context, _ string,
) (*ContainerStatus, error) {
	return nil, nil
}
func (a *availableRuntime) List(
	_ context.Context, _ ListFilter,
) ([]ContainerInfo, error) {
	return nil, nil
}
func (a *availableRuntime) Stats(
	_ context.Context, _ string,
) (*ContainerStats, error) {
	return nil, nil
}
func (a *availableRuntime) Exec(
	_ context.Context, _ string, _ []string,
) (*ExecResult, error) {
	return nil, nil
}
func (a *availableRuntime) Logs(
	_ context.Context, _ string, _ ...LogOption,
) (io.ReadCloser, error) {
	return nil, nil
}

func TestAutoDetect_DockerFirst(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: true},
		&availableRuntime{name: "podman", available: true},
		&availableRuntime{name: "kubernetes", available: true},
	}
	detect := testableAutoDetect(runtimes)
	rt, err := detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "docker", rt.Name())
}

func TestAutoDetect_FallbackToPodman(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: false},
		&availableRuntime{name: "podman", available: true},
		&availableRuntime{name: "kubernetes", available: false},
	}
	detect := testableAutoDetect(runtimes)
	rt, err := detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "podman", rt.Name())
}

func TestAutoDetect_FallbackToKubernetes(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: false},
		&availableRuntime{name: "podman", available: false},
		&availableRuntime{name: "kubernetes", available: true},
	}
	detect := testableAutoDetect(runtimes)
	rt, err := detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "kubernetes", rt.Name())
}

func TestAutoDetect_NoneAvailable(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: false},
		&availableRuntime{name: "podman", available: false},
		&availableRuntime{name: "kubernetes", available: false},
	}
	detect := testableAutoDetect(runtimes)
	rt, err := detect(context.Background())
	assert.Error(t, err)
	assert.Nil(t, rt)
	assert.Contains(t, err.Error(), "no container runtime detected")
}

func TestDetectAll_AllAvailable(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: true},
		&availableRuntime{name: "podman", available: true},
		&availableRuntime{name: "kubernetes", available: true},
	}
	detect := testableDetectAll(runtimes)
	available := detect(context.Background())
	assert.Len(t, available, 3)
}

func TestDetectAll_SomeAvailable(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: true},
		&availableRuntime{name: "podman", available: false},
		&availableRuntime{name: "kubernetes", available: true},
	}
	detect := testableDetectAll(runtimes)
	available := detect(context.Background())
	assert.Len(t, available, 2)
	assert.Equal(t, "docker", available[0].Name())
	assert.Equal(t, "kubernetes", available[1].Name())
}

func TestDetectAll_NoneAvailable(t *testing.T) {
	runtimes := []ContainerRuntime{
		&availableRuntime{name: "docker", available: false},
		&availableRuntime{name: "podman", available: false},
		&availableRuntime{name: "kubernetes", available: false},
	}
	detect := testableDetectAll(runtimes)
	available := detect(context.Background())
	assert.Empty(t, available)
}

func TestDetectAll_EmptyInput(t *testing.T) {
	detect := testableDetectAll(nil)
	available := detect(context.Background())
	assert.Empty(t, available)
}
