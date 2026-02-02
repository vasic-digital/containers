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

func TestKubernetesRuntime_Name(t *testing.T) {
	k := NewKubernetesRuntimeWithExecutor(&mockExecutor{}, "")
	assert.Equal(t, "kubernetes", k.Name())
}

func TestKubernetesRuntime_DefaultNamespace(t *testing.T) {
	k := NewKubernetesRuntimeWithExecutor(&mockExecutor{}, "")
	assert.Equal(t, "default", k.namespace)
}

func TestKubernetesRuntime_CustomNamespace(t *testing.T) {
	k := NewKubernetesRuntimeWithExecutor(
		&mockExecutor{}, "production",
	)
	assert.Equal(t, "production", k.namespace)
}

func TestKubernetesRuntime_Version(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		err     error
		want    string
		wantErr bool
	}{
		{
			name: "success",
			output: `{
				"serverVersion": {
					"gitVersion": "v1.28.4"
				}
			}`,
			want: "v1.28.4",
		},
		{
			name:    "command error",
			err:     fmt.Errorf("connection refused"),
			wantErr: true,
		},
		{
			name:    "invalid json",
			output:  "not json",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, error) {
					return []byte(tt.output), tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			got, err := k.Version(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKubernetesRuntime_IsAvailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"cluster reachable", nil, true},
		{"cluster unreachable", fmt.Errorf("refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, error) {
					return nil, tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			assert.Equal(t, tt.want,
				k.IsAvailable(context.Background()))
		})
	}
}

func TestKubernetesRuntime_Start(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"success", nil, false},
		{"error", fmt.Errorf("not found"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, args ...string,
				) ([]byte, error) {
					capturedArgs = args
					return nil, tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			err := k.Start(context.Background(), "my-deploy")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, capturedArgs, "scale")
			assert.Contains(t, capturedArgs, "--replicas=1")
			assert.Contains(t, capturedArgs,
				"deployment/my-deploy")
		})
	}
}

func TestKubernetesRuntime_Stop(t *testing.T) {
	var capturedArgs []string
	exec := &mockExecutor{
		executeFunc: func(
			_ context.Context, _ string, args ...string,
		) ([]byte, error) {
			capturedArgs = args
			return nil, nil
		},
	}
	k := NewKubernetesRuntimeWithExecutor(exec, "default")
	err := k.Stop(context.Background(), "my-deploy")
	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "scale")
	assert.Contains(t, capturedArgs, "--replicas=0")
}

func TestKubernetesRuntime_Remove(t *testing.T) {
	tests := []struct {
		name    string
		force   bool
		err     error
		wantErr bool
	}{
		{"simple delete", false, nil, false},
		{"force delete", true, nil, false},
		{"error", false, fmt.Errorf("fail"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, args ...string,
				) ([]byte, error) {
					capturedArgs = args
					return nil, tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			var opts []RemoveOption
			if tt.force {
				opts = append(opts, WithForceRemove(true))
			}
			err := k.Remove(
				context.Background(), "my-pod", opts...,
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, capturedArgs, "delete")
			assert.Contains(t, capturedArgs, "pod")
			if tt.force {
				assert.Contains(t, capturedArgs, "--force")
			}
		})
	}
}

func TestKubernetesRuntime_Status(t *testing.T) {
	podJSON := `{
		"metadata": {
			"name": "web-pod",
			"uid": "uid-123",
			"labels": {"app": "web"},
			"namespace": "default"
		},
		"spec": {
			"containers": [{
				"name": "web",
				"image": "nginx:latest",
				"ports": [{"containerPort": 80, "protocol": "TCP"}]
			}]
		},
		"status": {
			"phase": "Running",
			"startTime": "2024-01-15T10:00:00Z",
			"containerStatuses": [{
				"containerID": "containerd://abc123",
				"state": {
					"running": {
						"startedAt": "2024-01-15T10:00:05Z"
					}
				},
				"ready": true
			}]
		}
	}`

	tests := []struct {
		name    string
		output  string
		err     error
		wantErr bool
		wantID  string
		state   ContainerState
	}{
		{
			name:   "running pod",
			output: podJSON,
			wantID: "uid-123",
			state:  StateRunning,
		},
		{
			name:    "command error",
			err:     fmt.Errorf("not found"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, error) {
					return []byte(tt.output), tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			status, err := k.Status(
				context.Background(), "web-pod",
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, status.ID)
			assert.Equal(t, tt.state, status.State)
			assert.Equal(t, "web-pod", status.Name)
			assert.Equal(t, "healthy", status.Health)
		})
	}
}

func TestKubernetesRuntime_Status_TerminatedPod(t *testing.T) {
	podJSON := `{
		"metadata": {
			"name": "job-pod",
			"uid": "uid-456",
			"labels": {},
			"namespace": "default"
		},
		"spec": {
			"containers": [{
				"name": "worker",
				"image": "busybox"
			}]
		},
		"status": {
			"phase": "Succeeded",
			"startTime": "2024-01-15T10:00:00Z",
			"containerStatuses": [{
				"containerID": "containerd://def456",
				"state": {
					"terminated": {
						"exitCode": 0,
						"finishedAt": "2024-01-15T10:05:00Z"
					}
				},
				"ready": false
			}]
		}
	}`

	exec := &mockExecutor{
		executeFunc: func(
			_ context.Context, _ string, _ ...string,
		) ([]byte, error) {
			return []byte(podJSON), nil
		},
	}
	k := NewKubernetesRuntimeWithExecutor(exec, "default")
	status, err := k.Status(context.Background(), "job-pod")
	require.NoError(t, err)
	assert.Equal(t, StateStopped, status.State)
	assert.Equal(t, 0, status.ExitCode)
}

func TestKubernetesRuntime_List(t *testing.T) {
	podListJSON := `{
		"items": [
			{
				"metadata": {
					"name": "web-abc",
					"uid": "uid-1",
					"labels": {"app": "web"},
					"namespace": "default"
				},
				"spec": {
					"containers": [{
						"name": "web",
						"image": "nginx"
					}]
				},
				"status": {
					"phase": "Running",
					"startTime": "2024-01-15T10:00:00Z"
				}
			},
			{
				"metadata": {
					"name": "db-def",
					"uid": "uid-2",
					"labels": {"app": "db"},
					"namespace": "default"
				},
				"spec": {
					"containers": [{
						"name": "db",
						"image": "postgres"
					}]
				},
				"status": {
					"phase": "Running",
					"startTime": "2024-01-15T09:00:00Z"
				}
			}
		]
	}`

	tests := []struct {
		name    string
		filter  ListFilter
		output  string
		err     error
		wantErr bool
		wantLen int
	}{
		{
			name:    "all pods",
			output:  podListJSON,
			wantLen: 2,
		},
		{
			name: "filter by name",
			filter: ListFilter{
				Names: []string{"web"},
			},
			output:  podListJSON,
			wantLen: 1,
		},
		{
			name:    "command error",
			err:     fmt.Errorf("fail"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, error) {
					return []byte(tt.output), tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			containers, err := k.List(
				context.Background(), tt.filter,
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, containers, tt.wantLen)
		})
	}
}

func TestKubernetesRuntime_Stats(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		err     error
		wantErr bool
		wantCPU float64
	}{
		{
			name:    "success",
			output:  "web-pod   250m   128Mi",
			wantCPU: 25.0,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
		{
			name:    "command error",
			err:     fmt.Errorf("metrics unavailable"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, error) {
					return []byte(tt.output), tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			stats, err := k.Stats(
				context.Background(), "web-pod",
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.wantCPU, stats.CPUPercent, 0.1)
		})
	}
}

func TestKubernetesRuntime_Exec(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		stderr     string
		exitCode   int
		err        error
		wantErr    bool
		wantStdout string
	}{
		{
			name:       "success",
			stdout:     "hello from pod",
			exitCode:   0,
			wantStdout: "hello from pod",
		},
		{
			name:    "command error",
			err:     fmt.Errorf("pod not found"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			exec := &mockExecutor{
				executeWithStderrFunc: func(
					_ context.Context, _ string, args ...string,
				) ([]byte, []byte, int, error) {
					capturedArgs = args
					return []byte(tt.stdout),
						[]byte(tt.stderr),
						tt.exitCode,
						tt.err
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "testing")
			result, err := k.Exec(
				context.Background(),
				"my-pod",
				[]string{"echo", "hello"},
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.exitCode, result.ExitCode)
			assert.Equal(t, tt.wantStdout, result.Stdout)
			// Verify namespace is passed.
			assert.Contains(t, capturedArgs, "-n")
			assert.Contains(t, capturedArgs, "testing")
			assert.Contains(t, capturedArgs, "--")
		})
	}
}

func TestKubernetesRuntime_Logs(t *testing.T) {
	tests := []struct {
		name    string
		opts    []LogOption
		err     error
		wantErr bool
		content string
	}{
		{
			name:    "simple logs",
			content: "pod log output",
		},
		{
			name: "with follow and tail",
			opts: []LogOption{
				WithFollow(true),
				WithTail("50"),
			},
			content: "streaming logs",
		},
		{
			name:    "command error",
			err:     fmt.Errorf("fail"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeStreamFunc: func(
					_ context.Context, _ string, _ ...string,
				) (io.ReadCloser, error) {
					if tt.err != nil {
						return nil, tt.err
					}
					return io.NopCloser(
						strings.NewReader(tt.content),
					), nil
				},
			}
			k := NewKubernetesRuntimeWithExecutor(exec, "default")
			rc, err := k.Logs(
				context.Background(), "my-pod", tt.opts...,
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer rc.Close()
			data, readErr := io.ReadAll(rc)
			require.NoError(t, readErr)
			assert.Equal(t, tt.content, string(data))
		})
	}
}

func TestMapKubePhaseToState(t *testing.T) {
	tests := []struct {
		phase string
		want  ContainerState
	}{
		{"Running", StateRunning},
		{"running", StateRunning},
		{"Succeeded", StateStopped},
		{"Failed", StateStopped},
		{"Pending", StateCreated},
		{"Unknown", ContainerState("Unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			got := mapKubePhaseToState(tt.phase)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseKubeCPU(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"250m", 250},
		{"1", 1000},
		{"2", 2000},
		{"100m", 100},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseKubeCPU(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
