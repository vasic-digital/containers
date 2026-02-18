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

func TestRemoteComposeOrchestrator_Interface(t *testing.T) {
	var _ compose.ComposeOrchestrator = (*RemoteComposeOrchestrator)(nil)
}

func TestRemoteComposeOrchestrator_Up(t *testing.T) {
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	orch, exec := newTestComposeOrchestrator()
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
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "podman-host",
		Address: "192.168.1.101",
		User:    "deploy",
		Runtime: "podman",
	}

	exec.executeFunc = func(
		ctx context.Context, h RemoteHost, cmd string,
	) (*CommandResult, error) {
		assert.True(t, strings.HasPrefix(cmd, "podman compose"))
		return &CommandResult{ExitCode: 0}, nil
	}

	orch := NewRemoteComposeOrchestrator(
		host, exec, logging.NopLogger{},
	)

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
		name     string
		output   string
		expected int
	}{
		{
			"two services",
			"redis|running|healthy|6379/tcp|0\nnginx|exited||80/tcp|1\n",
			2,
		},
		{"empty output", "", 0},
		{"single line", "redis|running|healthy|6379/tcp|0", 1},
		{
			"incomplete line",
			"redis|running",
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statuses := parseRemoteComposeStatus(tt.output)
			assert.Len(t, statuses, tt.expected)
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
