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

func TestNerdctlRuntime_Exec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exec := &mockExecutor{
			executeWithStderrFunc: func(
				ctx context.Context, name string, args ...string,
			) ([]byte, []byte, int, error) {
				return []byte("stdout-output"), []byte(""), 0, nil
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		result, err := n.Exec(context.Background(), "ctr1", []string{"ls", "-la"})
		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Equal(t, "stdout-output", result.Stdout)
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		exec := &mockExecutor{
			executeWithStderrFunc: func(
				ctx context.Context, name string, args ...string,
			) ([]byte, []byte, int, error) {
				return []byte(""), []byte("err msg"), 2, nil
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		result, err := n.Exec(context.Background(), "ctr1", []string{"false"})
		require.NoError(t, err)
		assert.Equal(t, 2, result.ExitCode)
		assert.Equal(t, "err msg", result.Stderr)
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeWithStderrFunc: func(
				ctx context.Context, name string, args ...string,
			) ([]byte, []byte, int, error) {
				return nil, nil, -1, fmt.Errorf("exec failed")
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		_, err := n.Exec(context.Background(), "ctr1", []string{"cmd"})
		assert.Error(t, err)
	})
}

func TestNerdctlRuntime_Logs(t *testing.T) {
	t.Run("success no options", func(t *testing.T) {
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("log line")), nil
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		rc, err := n.Logs(context.Background(), "ctr1")
		require.NoError(t, err)
		defer rc.Close()
	})

	t.Run("success with all options", func(t *testing.T) {
		var capturedArgs []string
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				capturedArgs = args
				return io.NopCloser(strings.NewReader("log line")), nil
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		rc, err := n.Logs(context.Background(), "ctr1",
			WithFollow(true),
			WithSince("2024-01-01"),
			WithUntil("2024-12-31"),
			WithTail("100"),
		)
		require.NoError(t, err)
		defer rc.Close()
		assert.Contains(t, capturedArgs, "-f")
		assert.Contains(t, capturedArgs, "--since")
		assert.Contains(t, capturedArgs, "--until")
		assert.Contains(t, capturedArgs, "--tail")
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeStreamFunc: func(
				ctx context.Context, name string, args ...string,
			) (io.ReadCloser, error) {
				return nil, fmt.Errorf("stream failed")
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		_, err := n.Logs(context.Background(), "ctr1")
		assert.Error(t, err)
	})
}

func TestNerdctlRuntime_Remove_WithOptions(t *testing.T) {
	t.Run("force and volumes", func(t *testing.T) {
		var capturedArgs []string
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				capturedArgs = args
				return []byte(""), nil
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		err := n.Remove(context.Background(), "ctr1",
			WithForceRemove(true), WithRemoveVolumes(true))
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "-f")
		assert.Contains(t, capturedArgs, "-v")
	})

	t.Run("error", func(t *testing.T) {
		exec := &mockExecutor{
			executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return nil, fmt.Errorf("rm failed")
			},
		}
		n := NewNerdctlRuntimeWithExecutor(exec)
		err := n.Remove(context.Background(), "ctr1")
		assert.Error(t, err)
	})
}

func TestNerdctlRuntime_Status_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("inspect failed")
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	_, err := n.Status(context.Background(), "missing-ctr")
	assert.Error(t, err)
}

func TestNerdctlRuntime_List_WithFilters(t *testing.T) {
	output := `{"ID":"abc123","Names":"web","Image":"nginx","State":"running","Status":"Up","CreatedAt":"2024-01-15","Labels":"","Ports":""}`
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	containers, err := n.List(context.Background(), ListFilter{
		All:    true,
		Labels: map[string]string{"env": "prod"},
		Names:  []string{"web"},
		Status: []ContainerState{StateRunning},
	})
	require.NoError(t, err)
	assert.Len(t, containers, 1)
}

func TestNerdctlRuntime_Stats_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("stats failed")
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	_, err := n.Stats(context.Background(), "ctr1")
	assert.Error(t, err)
}

func TestParseNerdctlVersion_Empty(t *testing.T) {
	// parseCrioVersion-like fallback: invalid json returns raw trimmed
	ver, err := parseNerdctlVersion([]byte("bad-json"))
	require.NoError(t, err)
	assert.Equal(t, "bad-json", ver)
}
