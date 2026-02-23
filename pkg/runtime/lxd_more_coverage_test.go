//go:build !integration

package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLXDRuntime_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		output := `[{"name":"c1","status":"Running","status_code":103,"config":{}},{"name":"c2","status":"Stopped","status_code":102,"config":{}}]`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		containers, err := l.List(context.Background(), ListFilter{})
		require.NoError(t, err)
		assert.Len(t, containers, 2)
		assert.Equal(t, "c1", containers[0].ID)
		assert.Equal(t, StateRunning, containers[0].State)
		assert.Equal(t, StateStopped, containers[1].State)
	})

	t.Run("filter by status", func(t *testing.T) {
		output := `[{"name":"c1","status":"Running","status_code":103,"config":{}},{"name":"c2","status":"Stopped","status_code":102,"config":{}}]`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		containers, err := l.List(context.Background(), ListFilter{Status: []ContainerState{StateRunning}})
		require.NoError(t, err)
		assert.Len(t, containers, 1)
		assert.Equal(t, StateRunning, containers[0].State)
	})

	t.Run("filter by name", func(t *testing.T) {
		output := `[{"name":"web-app","status":"Running","status_code":103,"config":{}},{"name":"db-server","status":"Running","status_code":103,"config":{}}]`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		containers, err := l.List(context.Background(), ListFilter{Names: []string{"web"}})
		require.NoError(t, err)
		assert.Len(t, containers, 1)
		assert.Equal(t, "web-app", containers[0].Name)
	})

	t.Run("with user labels in config", func(t *testing.T) {
		output := `[{"name":"app","status":"Running","status_code":103,"config":{"user.env":"prod","image.alias":"ubuntu/20.04"}}]`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		containers, err := l.List(context.Background(), ListFilter{})
		require.NoError(t, err)
		require.Len(t, containers, 1)
		assert.Equal(t, "prod", containers[0].Labels["env"])
		assert.Equal(t, "ubuntu/20.04", containers[0].Image)
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return nil, fmt.Errorf("lxc list failed")
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		_, err := l.List(context.Background(), ListFilter{})
		assert.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("not-json"), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		_, err := l.List(context.Background(), ListFilter{})
		assert.Error(t, err)
	})
}

func TestLXDRuntime_Stats(t *testing.T) {
	t.Run("success with full state", func(t *testing.T) {
		output := `{"state":{"status":"Running","cpu":{"usage":100000},"memory":{"usage":52428800,"usage_peak":60000000,"limit":1073741824},"network":{"eth0":{"addresses":[],"counters":{"bytes_received":1024,"bytes_sent":512}}},"pid":42}}`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		stats, err := l.Stats(context.Background(), "my-container")
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(52428800), stats.MemoryUsage)
		assert.Equal(t, uint64(1073741824), stats.MemoryLimit)
		assert.InDelta(t, 4.88, stats.MemoryPercent, 0.01)
		assert.Equal(t, uint64(1024), stats.NetworkRx)
		assert.Equal(t, uint64(512), stats.NetworkTx)
		assert.Equal(t, 42, stats.PIDs)
	})

	t.Run("nil state returns empty stats", func(t *testing.T) {
		output := `{"state":null}`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		stats, err := l.Stats(context.Background(), "my-container")
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.MemoryUsage)
	})

	t.Run("zero memory limit prevents percentage calculation", func(t *testing.T) {
		output := `{"state":{"memory":{"usage":100,"limit":0},"network":{}}}`
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(output), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		stats, err := l.Stats(context.Background(), "my-container")
		require.NoError(t, err)
		assert.Equal(t, 0.0, stats.MemoryPercent)
	})

	t.Run("invalid json", func(t *testing.T) {
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("not-json"), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		_, err := l.Stats(context.Background(), "my-container")
		assert.Error(t, err)
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return nil, fmt.Errorf("info failed")
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		_, err := l.Stats(context.Background(), "my-container")
		assert.Error(t, err)
	})
}

func TestLXDRuntime_Exec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exec := &mockExecutor{
			executeWithStderrFunc: func(
				ctx context.Context, name string, args ...string,
			) ([]byte, []byte, int, error) {
				return []byte("hello"), []byte(""), 0, nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		result, err := l.Exec(context.Background(), "my-container", []string{"echo", "hello"})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Equal(t, "hello", result.Stdout)
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		exec := &mockExecutor{
			executeWithStderrFunc: func(
				ctx context.Context, name string, args ...string,
			) ([]byte, []byte, int, error) {
				return []byte(""), []byte("not found"), 127, nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		result, err := l.Exec(context.Background(), "my-container", []string{"missing"})
		require.NoError(t, err)
		assert.Equal(t, 127, result.ExitCode)
		assert.Equal(t, "not found", result.Stderr)
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeWithStderrFunc: func(
				ctx context.Context, name string, args ...string,
			) ([]byte, []byte, int, error) {
				return nil, nil, -1, fmt.Errorf("exec failed")
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		_, err := l.Exec(context.Background(), "my-container", []string{"cmd"})
		assert.Error(t, err)
	})
}

func TestLXDRuntime_Logs(t *testing.T) {
	t.Run("success no options", func(t *testing.T) {
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("log output")), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		rc, err := l.Logs(context.Background(), "my-container")
		require.NoError(t, err)
		defer rc.Close()
	})

	t.Run("success with tail option", func(t *testing.T) {
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("log output")), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		rc, err := l.Logs(context.Background(), "my-container", WithTail("50"))
		require.NoError(t, err)
		defer rc.Close()
	})

	t.Run("success with invalid tail (non-numeric)", func(t *testing.T) {
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("log output")), nil
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		rc, err := l.Logs(context.Background(), "my-container", WithTail("all"))
		require.NoError(t, err)
		defer rc.Close()
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				return nil, fmt.Errorf("stream failed")
			},
		}
		l := NewLXDRuntimeWithExecutor(exec)
		_, err := l.Logs(context.Background(), "my-container")
		assert.Error(t, err)
	})
}

func TestMapLXDStatusToState(t *testing.T) {
	assert.Equal(t, StateRunning, mapLXDStatusToState("Running", 103))
	assert.Equal(t, StateRunning, mapLXDStatusToState("running", 0))
	assert.Equal(t, StateStopped, mapLXDStatusToState("Stopped", 102))
	assert.Equal(t, StateStopped, mapLXDStatusToState("stopped", 0))
	assert.Equal(t, StatePaused, mapLXDStatusToState("Frozen", 101))
	assert.Equal(t, StatePaused, mapLXDStatusToState("frozen", 0))
	assert.Equal(t, StateRestarting, mapLXDStatusToState("Starting", 106))
	assert.Equal(t, StateRestarting, mapLXDStatusToState("starting", 0))
	assert.Equal(t, ContainerState("CustomState"), mapLXDStatusToState("CustomState", 999))
}

func TestLXDRuntime_Remove_Force(t *testing.T) {
	var capturedArgs []string
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte(""), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	err := l.Remove(context.Background(), "my-container", WithForceRemove(true))
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "--force")
}
