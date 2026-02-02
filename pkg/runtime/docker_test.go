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

// mockExecutor provides a mock CommandExecutor for testing.
type mockExecutor struct {
	executeFunc           func(ctx context.Context, name string, args ...string) ([]byte, error)
	executeWithStderrFunc func(
		ctx context.Context, name string, args ...string,
	) ([]byte, []byte, int, error)
	executeStreamFunc func(
		ctx context.Context, name string, args ...string,
	) (io.ReadCloser, error)
}

func (m *mockExecutor) Execute(
	ctx context.Context, name string, args ...string,
) ([]byte, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, args...)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockExecutor) ExecuteWithStderr(
	ctx context.Context, name string, args ...string,
) ([]byte, []byte, int, error) {
	if m.executeWithStderrFunc != nil {
		return m.executeWithStderrFunc(ctx, name, args...)
	}
	return nil, nil, 0, fmt.Errorf("not implemented")
}

func (m *mockExecutor) ExecuteStream(
	ctx context.Context, name string, args ...string,
) (io.ReadCloser, error) {
	if m.executeStreamFunc != nil {
		return m.executeStreamFunc(ctx, name, args...)
	}
	return nil, fmt.Errorf("not implemented")
}

func TestDockerRuntime_Name(t *testing.T) {
	d := NewDockerRuntimeWithExecutor(&mockExecutor{})
	assert.Equal(t, "docker", d.Name())
}

func TestDockerRuntime_Version(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		err     error
		want    string
		wantErr bool
	}{
		{
			name:   "success",
			output: "24.0.7\n",
			want:   "24.0.7",
		},
		{
			name:    "error",
			err:     fmt.Errorf("command failed"),
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
			d := NewDockerRuntimeWithExecutor(exec)
			got, err := d.Version(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDockerRuntime_IsAvailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"available", nil, true},
		{"not available", fmt.Errorf("not found"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, error) {
					return []byte("ok"), tt.err
				},
			}
			d := NewDockerRuntimeWithExecutor(exec)
			got := d.IsAvailable(context.Background())
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDockerRuntime_Start(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"success", nil, false},
		{"error", fmt.Errorf("fail"), true},
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
			d := NewDockerRuntimeWithExecutor(exec)
			err := d.Start(context.Background(), "test-container")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, capturedArgs, "start")
			assert.Contains(t, capturedArgs, "test-container")
		})
	}
}

func TestDockerRuntime_Stop(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		err     error
		wantErr bool
	}{
		{"success with default timeout", "10", nil, false},
		{"error", "10", fmt.Errorf("fail"), true},
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
			d := NewDockerRuntimeWithExecutor(exec)
			err := d.Stop(context.Background(), "test-container")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, capturedArgs, "stop")
			assert.Contains(t, capturedArgs, "-t")
			assert.Contains(t, capturedArgs, tt.timeout)
		})
	}
}

func TestDockerRuntime_Remove(t *testing.T) {
	tests := []struct {
		name       string
		force      bool
		volumes    bool
		err        error
		wantErr    bool
		wantForce  bool
		wantVolume bool
	}{
		{
			name: "simple remove",
		},
		{
			name:      "force remove",
			force:     true,
			wantForce: true,
		},
		{
			name:       "force with volumes",
			force:      true,
			volumes:    true,
			wantForce:  true,
			wantVolume: true,
		},
		{
			name:    "error",
			err:     fmt.Errorf("fail"),
			wantErr: true,
		},
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
			d := NewDockerRuntimeWithExecutor(exec)
			var opts []RemoveOption
			if tt.force {
				opts = append(opts, WithForceRemove(true))
			}
			if tt.volumes {
				opts = append(opts, WithRemoveVolumes(true))
			}
			err := d.Remove(
				context.Background(), "test-container", opts...,
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, capturedArgs, "rm")
			if tt.wantForce {
				assert.Contains(t, capturedArgs, "-f")
			}
			if tt.wantVolume {
				assert.Contains(t, capturedArgs, "-v")
			}
		})
	}
}

func TestDockerRuntime_Status(t *testing.T) {
	inspectJSON := `[{
		"Id": "abc123",
		"Name": "/my-container",
		"State": {
			"Status": "running",
			"Running": true,
			"ExitCode": 0,
			"StartedAt": "2024-01-15T10:30:00Z",
			"FinishedAt": "0001-01-01T00:00:00Z"
		},
		"Config": {"Labels": {}, "Image": "nginx"},
		"Image": "sha256:abc",
		"Created": "2024-01-15T10:00:00Z",
		"NetworkSettings": {
			"Networks": {},
			"Ports": {
				"80/tcp": [{"HostIp": "0.0.0.0", "HostPort": "8080"}]
			}
		}
	}]`

	tests := []struct {
		name    string
		output  string
		err     error
		wantErr bool
		wantID  string
		state   ContainerState
	}{
		{
			name:   "running container",
			output: inspectJSON,
			wantID: "abc123",
			state:  StateRunning,
		},
		{
			name:    "command error",
			err:     fmt.Errorf("not found"),
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
			d := NewDockerRuntimeWithExecutor(exec)
			status, err := d.Status(
				context.Background(), "my-container",
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, status.ID)
			assert.Equal(t, tt.state, status.State)
			assert.Equal(t, "my-container", status.Name)
		})
	}
}

func TestDockerRuntime_List(t *testing.T) {
	psLine1 := `{"ID":"abc123","Names":"web","Image":"nginx","State":"running","Status":"Up 2 hours","CreatedAt":"2024-01-15","Labels":"app=web","Ports":"80/tcp"}`
	psLine2 := `{"ID":"def456","Names":"db","Image":"postgres","State":"running","Status":"Up 1 hour","CreatedAt":"2024-01-15","Labels":"app=db","Ports":"5432/tcp"}`

	tests := []struct {
		name    string
		output  string
		filter  ListFilter
		err     error
		wantErr bool
		wantLen int
	}{
		{
			name:    "two containers",
			output:  psLine1 + "\n" + psLine2,
			wantLen: 2,
		},
		{
			name:    "empty output",
			output:  "",
			wantLen: 0,
		},
		{
			name: "with all filter",
			filter: ListFilter{
				All: true,
			},
			output:  psLine1,
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
			var capturedArgs []string
			exec := &mockExecutor{
				executeFunc: func(
					_ context.Context, _ string, args ...string,
				) ([]byte, error) {
					capturedArgs = args
					return []byte(tt.output), tt.err
				},
			}
			d := NewDockerRuntimeWithExecutor(exec)
			containers, err := d.List(
				context.Background(), tt.filter,
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, containers, tt.wantLen)
			if tt.filter.All {
				assert.Contains(t, capturedArgs, "-a")
			}
		})
	}
}

func TestDockerRuntime_Stats(t *testing.T) {
	statsJSON := `{"CPUPerc":"0.50%","MemPerc":"12.34%","MemUsage":"100MiB / 1GiB","NetIO":"1kB / 2kB","BlockIO":"500B / 1kB","PIDs":"10"}`

	tests := []struct {
		name    string
		output  string
		err     error
		wantErr bool
		wantCPU float64
		wantMem float64
	}{
		{
			name:    "success",
			output:  statsJSON,
			wantCPU: 0.50,
			wantMem: 12.34,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
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
			d := NewDockerRuntimeWithExecutor(exec)
			stats, err := d.Stats(
				context.Background(), "test-container",
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.wantCPU, stats.CPUPercent, 0.01)
			assert.InDelta(t, tt.wantMem, stats.MemoryPercent, 0.01)
			assert.Equal(t, 10, stats.PIDs)
		})
	}
}

func TestDockerRuntime_Exec(t *testing.T) {
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
			stdout:     "hello world",
			stderr:     "",
			exitCode:   0,
			wantStdout: "hello world",
		},
		{
			name:     "non-zero exit",
			stdout:   "",
			stderr:   "error occurred",
			exitCode: 1,
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
				executeWithStderrFunc: func(
					_ context.Context, _ string, _ ...string,
				) ([]byte, []byte, int, error) {
					return []byte(tt.stdout),
						[]byte(tt.stderr),
						tt.exitCode,
						tt.err
				},
			}
			d := NewDockerRuntimeWithExecutor(exec)
			result, err := d.Exec(
				context.Background(),
				"test-container",
				[]string{"echo", "hello"},
			)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.exitCode, result.ExitCode)
			if tt.wantStdout != "" {
				assert.Equal(t, tt.wantStdout, result.Stdout)
			}
		})
	}
}

func TestDockerRuntime_Logs(t *testing.T) {
	tests := []struct {
		name    string
		opts    []LogOption
		err     error
		wantErr bool
		content string
	}{
		{
			name:    "simple logs",
			content: "log line 1\nlog line 2\n",
		},
		{
			name:    "with follow",
			opts:    []LogOption{WithFollow(true)},
			content: "streaming...",
		},
		{
			name:    "with since and tail",
			opts:    []LogOption{WithSince("1h"), WithTail("50")},
			content: "recent logs",
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
			d := NewDockerRuntimeWithExecutor(exec)
			rc, err := d.Logs(
				context.Background(),
				"test-container",
				tt.opts...,
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

func TestDockerRuntime_Status_PortParsing(t *testing.T) {
	inspectJSON := `[{
		"Id": "abc123",
		"Name": "/my-container",
		"State": {
			"Status": "running",
			"Running": true,
			"ExitCode": 0,
			"StartedAt": "2024-01-15T10:30:00Z",
			"FinishedAt": "0001-01-01T00:00:00Z"
		},
		"Config": {"Labels": {}, "Image": "nginx"},
		"Image": "sha256:abc",
		"Created": "2024-01-15T10:00:00Z",
		"NetworkSettings": {
			"Networks": {},
			"Ports": {
				"80/tcp": [{"HostIp": "0.0.0.0", "HostPort": "8080"}],
				"443/tcp": [{"HostIp": "0.0.0.0", "HostPort": "8443"}]
			}
		}
	}]`

	exec := &mockExecutor{
		executeFunc: func(
			_ context.Context, _ string, _ ...string,
		) ([]byte, error) {
			return []byte(inspectJSON), nil
		},
	}
	d := NewDockerRuntimeWithExecutor(exec)
	status, err := d.Status(context.Background(), "my-container")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(status.Ports), 2)
	foundHTTP := false
	foundHTTPS := false
	for _, p := range status.Ports {
		if p.ContainerPort == "80" && p.HostPort == "8080" {
			foundHTTP = true
		}
		if p.ContainerPort == "443" && p.HostPort == "8443" {
			foundHTTPS = true
		}
	}
	assert.True(t, foundHTTP, "expected port 80 mapping")
	assert.True(t, foundHTTPS, "expected port 443 mapping")
}
