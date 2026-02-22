package remote

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/logging"
)

// newTestComposeOrchestrator creates a test orchestrator with auto-detection.
// The mock executor will simulate podman-compose being available.
func newTestComposeOrchestrator() (
	*RemoteComposeOrchestrator, *mockExecutor,
) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
		Runtime: "docker",
	}
	return NewRemoteComposeOrchestrator(
		host, exec, logging.NopLogger{},
	), exec
}

// newTestComposeOrchestratorWithCommand creates a test orchestrator with a forced compose command.
// This bypasses auto-detection for simpler testing.
func newTestComposeOrchestratorWithCommand(cmd string) (
	*RemoteComposeOrchestrator, *mockExecutor,
) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
		Runtime: "docker",
	}
	return NewRemoteComposeOrchestrator(
		host, exec, logging.NopLogger{},
		WithComposeCommand(cmd),
	), exec
}

func TestRemoteComposeOrchestrator_Interface(t *testing.T) {
	var _ compose.ComposeOrchestrator = (*RemoteComposeOrchestrator)(nil)
}

func TestRemoteComposeOrchestrator_Up(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "docker compose")
		assert.Contains(t, cmd, "up -d")
		assert.Contains(t, cmd, "-f docker-compose.yml")
		return &CommandResult{ExitCode: 0}, nil
	}

	project := compose.ComposeProject{
		File: "docker-compose.yml",
	}
	err := orch.Up(context.Background(), project)
	assert.NoError(t, err)
}

func TestRemoteComposeOrchestrator_Up_WithProfile(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "--profile monitoring")
		return &CommandResult{ExitCode: 0}, nil
	}

	project := compose.ComposeProject{
		File:    "docker-compose.yml",
		Profile: "monitoring",
	}
	err := orch.Up(context.Background(), project)
	assert.NoError(t, err)
}

func TestRemoteComposeOrchestrator_Up_WithServices(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "redis nginx")
		return &CommandResult{ExitCode: 0}, nil
	}

	project := compose.ComposeProject{
		File:     "docker-compose.yml",
		Services: []string{"redis", "nginx"},
	}
	err := orch.Up(context.Background(), project)
	assert.NoError(t, err)
}

func TestRemoteComposeOrchestrator_Up_Error(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return nil, fmt.Errorf("connection refused")
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Up(context.Background(), project)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestRemoteComposeOrchestrator_Up_NonZeroExit(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			ExitCode: 1,
			Stderr:   "service not found",
		}, nil
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Up(context.Background(), project)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
}

func TestRemoteComposeOrchestrator_Down(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.Contains(t, cmd, "down")
		return &CommandResult{ExitCode: 0}, nil
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Down(context.Background(), project)
	assert.NoError(t, err)
}

func TestRemoteComposeOrchestrator_Down_Error(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			ExitCode: 1,
			Stderr:   "not running",
		}, nil
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Down(context.Background(), project)
	assert.Error(t, err)
}

func TestRemoteComposeOrchestrator_Status(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return &CommandResult{
			Stdout: "redis|running|healthy|6379/tcp|0\n" +
				"nginx|running|healthy|80/tcp|0\n",
		}, nil
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	statuses, err := orch.Status(context.Background(), project)
	require.NoError(t, err)
	assert.Len(t, statuses, 2)
	assert.Equal(t, "redis", statuses[0].Name)
	assert.Equal(t, "running", statuses[0].State)
	assert.Equal(t, "nginx", statuses[1].Name)
}

func TestRemoteComposeOrchestrator_Status_Error(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		return nil, fmt.Errorf("timeout")
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	_, err := orch.Status(context.Background(), project)
	assert.Error(t, err)
}

func TestRemoteComposeOrchestrator_Logs(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("docker compose")
	exec.executeStreamFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (io.ReadCloser, error) {
		assert.Contains(t, cmd, "logs")
		assert.Contains(t, cmd, "redis")
		return io.NopCloser(
			strings.NewReader("redis log output\n"),
		), nil
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	reader, err := orch.Logs(
		context.Background(), project, "redis",
	)
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(data), "redis log output")
}

func TestRemoteComposeOrchestrator_Podman(t *testing.T) {
	orch, exec := newTestComposeOrchestratorWithCommand("podman-compose")
	exec.executeFunc = func(
		ctx context.Context, h RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.True(t, strings.HasPrefix(cmd, "podman-compose"))
		return &CommandResult{ExitCode: 0}, nil
	}

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Up(context.Background(), project)
	assert.NoError(t, err)
}

// TestRemoteComposeOrchestrator_AutoDetection tests that auto-detection works correctly
func TestRemoteComposeOrchestrator_AutoDetection(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
		Runtime: "podman",
	}

	callCount := 0
	exec.executeFunc = func(
		ctx context.Context, h RemoteHost, cmd string,
	) (*CommandResult, error) {
		callCount++
		// First call is detection: podman-compose version --short
		if strings.Contains(cmd, "version --short") {
			// Simulate podman-compose being available
			if strings.HasPrefix(cmd, "podman-compose") {
				return &CommandResult{ExitCode: 0, Stdout: "1.0.6"}, nil
			}
			return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
		}
		// Second call is compose up
		assert.True(t, strings.HasPrefix(cmd, "podman-compose -f"))
		assert.Contains(t, cmd, "up -d")
		return &CommandResult{ExitCode: 0}, nil
	}

	orch := NewRemoteComposeOrchestrator(host, exec, logging.NopLogger{})

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Up(context.Background(), project)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "should call detection then compose up")
}

// TestRemoteComposeOrchestrator_AutoDetection_Fallback tests fallback when podman-compose not found
func TestRemoteComposeOrchestrator_AutoDetection_Fallback(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
		Runtime: "podman",
	}

	exec.executeFunc = func(
		ctx context.Context, h RemoteHost, cmd string,
	) (*CommandResult, error) {
		// Detection phase: only docker compose is available
		if strings.Contains(cmd, "version --short") {
			if strings.HasPrefix(cmd, "docker compose") {
				return &CommandResult{ExitCode: 0, Stdout: "2.24.0"}, nil
			}
			return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
		}
		// Compose up with docker compose
		assert.True(t, strings.HasPrefix(cmd, "docker compose -f"))
		return &CommandResult{ExitCode: 0}, nil
	}

	orch := NewRemoteComposeOrchestrator(host, exec, logging.NopLogger{})

	project := compose.ComposeProject{File: "docker-compose.yml"}
	err := orch.Up(context.Background(), project)
	assert.NoError(t, err)
}

func TestRemoteComposeOrchestrator_Host(t *testing.T) {
	orch, _ := newTestComposeOrchestrator()
	host := orch.Host()
	assert.Equal(t, "test-host", host.Name)
}

func TestParseRemoteComposeStatus(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expected      int
		expectedNames []string
		expectedState []string
	}{
		{
			"two services (3-field format)",
			"redis|running|Up 2 hours (healthy)\nnginx|exited|Exited (0) 1 hour ago\n",
			2,
			[]string{"redis", "nginx"},
			[]string{"running", "exited"},
		},
		{
			"single service with health",
			"postgres|running|Up 5 minutes (healthy)",
			1,
			[]string{"postgres"},
			[]string{"running"},
		},
		{
			"service unhealthy",
			"api|running|Up 10 minutes (unhealthy)",
			1,
			[]string{"api"},
			[]string{"running"},
		},
		{"empty output", "", 0, nil, nil},
		{
			"incomplete line (only 1 field)",
			"redis",
			0,
			nil,
			nil,
		},
		{
			"complete 2-field line",
			"redis|running",
			1,
			[]string{"redis"},
			[]string{"running"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statuses := parseRemoteComposeStatus(tt.output)
			assert.Len(t, statuses, tt.expected)
			if tt.expected > 0 {
				for i, s := range statuses {
					assert.Equal(t, tt.expectedNames[i], s.Name)
					assert.Equal(t, tt.expectedState[i], s.State)
				}
			}
		})
	}
}

func TestRemoteComposeOrchestrator_ProjectArgs(t *testing.T) {
	orch, _ := newTestComposeOrchestrator()

	tests := []struct {
		name    string
		project compose.ComposeProject
		want    []string
	}{
		{
			"file only",
			compose.ComposeProject{File: "dc.yml"},
			[]string{"-f", "dc.yml"},
		},
		{
			"file and project name",
			compose.ComposeProject{
				File: "dc.yml", Name: "myproject",
			},
			[]string{"-f", "dc.yml", "--project-name", "myproject"},
		},
		{
			"all fields",
			compose.ComposeProject{
				File:    "dc.yml",
				Name:    "myproject",
				Profile: "dev",
			},
			[]string{
				"-f", "dc.yml",
				"--project-name", "myproject",
				"--profile", "dev",
			},
		},
		{
			"empty",
			compose.ComposeProject{},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := orch.projectArgs(tt.project)
			assert.Equal(t, tt.want, args)
		})
	}
}
