package remote

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

func newTestRemoteRuntime() (*RemoteRuntime, *mockExecutor) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
		Runtime: "docker",
	}
	return NewRemoteRuntime(host, exec, logging.NopLogger{}), exec
}

func TestRemoteRuntime_Interface(t *testing.T) {
	// Verify RemoteRuntime satisfies runtime.ContainerRuntime.
	var _ runtime.ContainerRuntime = (*RemoteRuntime)(nil)
}

func TestRemoteRuntime_Name(t *testing.T) {
	rt, _ := newTestRemoteRuntime()
	assert.Equal(t, "remote:test-host:docker", rt.Name())
}

func TestRemoteRuntime_Name_Podman(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "podman-host",
		Address: "192.168.1.101",
		User:    "deploy",
		Runtime: "podman",
	}
	rt := NewRemoteRuntime(host, exec, nil)
	assert.Equal(t, "remote:podman-host:podman", rt.Name())
}

func TestRemoteRuntime_Version(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "docker version")
		return &CommandResult{
			Stdout: "24.0.7\n",
		}, nil
	}

	version, err := rt.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "24.0.7", version)
}

func TestRemoteRuntime_IsAvailable(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			Stdout:   "abc123\n",
			ExitCode: 0,
		}, nil
	}

	assert.True(t, rt.IsAvailable(context.Background()))
}

func TestRemoteRuntime_IsAvailable_Error(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return nil, fmt.Errorf("connection refused")
	}

	assert.False(t, rt.IsAvailable(context.Background()))
}

func TestRemoteRuntime_Start(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "docker start my-container")
		return &CommandResult{ExitCode: 0}, nil
	}

	err := rt.Start(context.Background(), "my-container")
	assert.NoError(t, err)
}

func TestRemoteRuntime_Stop(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "docker stop my-container")
		return &CommandResult{ExitCode: 0}, nil
	}

	err := rt.Stop(context.Background(), "my-container")
	assert.NoError(t, err)
}

func TestRemoteRuntime_Remove(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "docker rm -f my-container")
		return &CommandResult{ExitCode: 0}, nil
	}

	err := rt.Remove(context.Background(), "my-container")
	assert.NoError(t, err)
}

func TestRemoteRuntime_Status(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			Stdout: "abc123|/my-container|running|healthy|" +
				"2024-01-01T00:00:00Z|0001-01-01T00:00:00Z|0",
		}, nil
	}

	status, err := rt.Status(
		context.Background(), "my-container",
	)
	require.NoError(t, err)
	assert.Equal(t, "abc123", status.ID)
	assert.Equal(t, "my-container", status.Name)
	assert.Equal(t, runtime.StateRunning, status.State)
	assert.Equal(t, "healthy", status.Health)
}

func TestRemoteRuntime_List(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			Stdout: `{"ID":"abc","Names":"test","Image":"nginx","State":"running","Status":"Up 5 min"}` + "\n",
		}, nil
	}

	containers, err := rt.List(
		context.Background(), runtime.ListFilter{All: true},
	)
	require.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, "abc", containers[0].ID)
	assert.Equal(t, "test", containers[0].Name)
}

func TestRemoteRuntime_List_Empty(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{Stdout: ""}, nil
	}

	containers, err := rt.List(
		context.Background(), runtime.ListFilter{},
	)
	require.NoError(t, err)
	assert.Nil(t, containers)
}

func TestRemoteRuntime_Stats(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			Stdout: "25.50%|10.20%|256MiB/2GiB|1.2MB/500kB|50MB/10MB|42",
		}, nil
	}

	stats, err := rt.Stats(
		context.Background(), "my-container",
	)
	require.NoError(t, err)
	assert.InDelta(t, 25.50, stats.CPUPercent, 0.01)
	assert.InDelta(t, 10.20, stats.MemoryPercent, 0.01)
	assert.Equal(t, 42, stats.PIDs)
}

func TestRemoteRuntime_Exec(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "exec my-container")
		return &CommandResult{
			Stdout:   "hello world\n",
			ExitCode: 0,
		}, nil
	}

	result, err := rt.Exec(
		context.Background(),
		"my-container",
		[]string{"echo", "hello world"},
	)
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestRemoteRuntime_Logs(t *testing.T) {
	rt, exec := newTestRemoteRuntime()
	exec.executeStreamFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (io.ReadCloser, error) {
		return io.NopCloser(
			strings.NewReader("log line 1\nlog line 2\n"),
		), nil
	}

	reader, err := rt.Logs(
		context.Background(), "my-container",
	)
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(data), "log line 1")
}

func TestRemoteRuntime_Host(t *testing.T) {
	rt, _ := newTestRemoteRuntime()
	host := rt.Host()
	assert.Equal(t, "test-host", host.Name)
	assert.Equal(t, "192.168.1.100", host.Address)
}

func TestRemoteRuntime_DefaultRuntime(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test",
		Address: "192.168.1.100",
		User:    "deploy",
		Runtime: "", // empty -> default to docker
	}

	exec.executeFunc = func(
		ctx context.Context, h RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.True(t, strings.HasPrefix(cmd, "docker "))
		return &CommandResult{ExitCode: 0}, nil
	}

	rt := NewRemoteRuntime(host, exec, nil)
	_ = rt.Start(context.Background(), "test")
}

func TestParseRemoteStatus_InvalidOutput(t *testing.T) {
	_, err := parseRemoteStatus("invalid")
	assert.Error(t, err)
}

func TestParseContainerList_InvalidJSON(t *testing.T) {
	containers, err := parseContainerList("not json")
	assert.NoError(t, err)
	assert.Empty(t, containers)
}

func TestParseRemoteStats_InvalidOutput(t *testing.T) {
	_, err := parseRemoteStats("invalid")
	assert.Error(t, err)
}
