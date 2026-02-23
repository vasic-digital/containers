//go:build !integration

package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNerdctlRuntime_Name(t *testing.T) {
	n := NewNerdctlRuntimeWithExecutor(&mockExecutor{})
	assert.Equal(t, "nerdctl", n.Name())
}

func TestNerdctlRuntime_Version(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
		want    string
	}{
		{
			name:   "success",
			output: `{"Client":{"Version":"1.7.3"},"Server":{"Version":"1.7.3"}}`,
			want:   "1.7.3",
		},
		{
			name:    "error",
			wantErr: true,
		},
		{
			name:    "bad json falls back to raw string",
			output:  "1.7.3",
			wantErr: false,
			want:    "1.7.3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
					if tt.wantErr && tt.output == "" {
						return nil, fmt.Errorf("command failed")
					}
					return []byte(tt.output), nil
				},
			}
			n := NewNerdctlRuntimeWithExecutor(exec)
			ver, err := n.Version(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, ver)
			}
		})
	}
}

func TestNerdctlRuntime_IsAvailable(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("[]"), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	assert.True(t, n.IsAvailable(context.Background()))

	exec2 := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	n2 := NewNerdctlRuntimeWithExecutor(exec2)
	assert.False(t, n2.IsAvailable(context.Background()))
}

func TestNerdctlRuntime_Start(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	err := n.Start(context.Background(), "test-container")
	assert.NoError(t, err)
}

func TestNerdctlRuntime_Start_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("start failed")
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	err := n.Start(context.Background(), "test-container")
	assert.Error(t, err)
}

func TestNerdctlRuntime_Stop(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	err := n.Stop(context.Background(), "test-container")
	assert.NoError(t, err)
}

func TestNerdctlRuntime_Remove(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	err := n.Remove(context.Background(), "test-container")
	assert.NoError(t, err)
}

func TestNerdctlRuntime_Status(t *testing.T) {
	// nerdctl uses docker inspect format (JSON array with State.Status field)
	output := `[{"Id":"abc123def456","Name":"/web","State":{"Status":"running","Running":true,"ExitCode":0,"StartedAt":"2024-01-15T10:30:00Z","FinishedAt":"0001-01-01T00:00:00Z"},"Config":{"Labels":{},"Image":"nginx"},"Image":"sha256:abc","Created":"2024-01-15T10:00:00Z","NetworkSettings":{"Networks":{},"Ports":{}}}]`
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	status, err := n.Status(context.Background(), "abc123def456")
	require.NoError(t, err)
	assert.Equal(t, StateRunning, status.State)
}

func TestNerdctlRuntime_List(t *testing.T) {
	// nerdctl uses docker ps JSON format (one JSON object per line)
	output := `{"ID":"abc123","Names":"web","Image":"nginx","State":"running","Status":"Up","CreatedAt":"2024-01-15","Labels":"","Ports":""}`
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	containers, err := n.List(context.Background(), ListFilter{})
	require.NoError(t, err)
	assert.Len(t, containers, 1)
}

func TestNerdctlRuntime_Stats(t *testing.T) {
	output := `{"CPUPerc":"5.0%","MemUsage":"100MiB / 1GiB","MemPerc":"10.0%","NetIO":"1MiB / 500KiB","BlockIO":"10MiB / 5MiB","PIDs":"5"}`
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)
	stats, err := n.Stats(context.Background(), "test-container")
	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestNerdctlRuntime_NewNerdctlRuntime(t *testing.T) {
	n := NewNerdctlRuntime()
	assert.NotNil(t, n)
	assert.Equal(t, "nerdctl", n.binary)
}
