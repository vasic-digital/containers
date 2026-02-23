//go:build !integration

package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLXDRuntime_Name(t *testing.T) {
	l := NewLXDRuntimeWithExecutor(&mockExecutor{})
	assert.Equal(t, "lxd", l.Name())
}

func TestLXDRuntime_Version(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("5.19\n"), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	ver, err := l.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "5.19", ver)
}

func TestLXDRuntime_Version_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("lxc not found")
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	_, err := l.Version(context.Background())
	assert.Error(t, err)
}

func TestLXDRuntime_IsAvailable(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("[]"), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	assert.True(t, l.IsAvailable(context.Background()))

	exec2 := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("not available")
		},
	}
	l2 := NewLXDRuntimeWithExecutor(exec2)
	assert.False(t, l2.IsAvailable(context.Background()))
}

func TestLXDRuntime_Start(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	err := l.Start(context.Background(), "my-container")
	assert.NoError(t, err)
}

func TestLXDRuntime_Start_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("start failed")
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	err := l.Start(context.Background(), "my-container")
	assert.Error(t, err)
}

func TestLXDRuntime_Stop(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	err := l.Stop(context.Background(), "my-container")
	assert.NoError(t, err)
}

func TestLXDRuntime_Remove(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	err := l.Remove(context.Background(), "my-container")
	assert.NoError(t, err)
}

func TestLXDRuntime_Status(t *testing.T) {
	// parseLXDStatus expects a JSON array of lxdContainerJSON objects
	// with top-level "Status" field (not nested in "State")
	output := `[{"name":"my-container","status":"Running","status_code":103,"state":{"status":"Running","cpu":{"usage":100000},"memory":{"usage":52428800,"limit":0}}}]`
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	status, err := l.Status(context.Background(), "my-container")
	require.NoError(t, err)
	assert.Equal(t, StateRunning, status.State)
}

func TestLXDRuntime_Status_Stopped(t *testing.T) {
	// status_code 102 maps to StateStopped
	output := `[{"name":"my-container","status":"Stopped","status_code":102,"state":{"status":"Stopped"}}]`
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(output), nil
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	status, err := l.Status(context.Background(), "my-container")
	require.NoError(t, err)
	assert.Equal(t, StateStopped, status.State)
}

func TestLXDRuntime_Status_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	l := NewLXDRuntimeWithExecutor(exec)
	_, err := l.Status(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestLXDRuntime_NewLXDRuntime(t *testing.T) {
	l := NewLXDRuntime()
	assert.NotNil(t, l)
	assert.Equal(t, "lxc", l.binary)
}
