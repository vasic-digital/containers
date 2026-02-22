package remote

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
)

func TestComposeCommand_String(t *testing.T) {
	tests := []struct {
		name     string
		cmd      ComposeCommand
		expected string
	}{
		{
			"podman-compose",
			ComposeCommand{Binary: "podman-compose", Subcommand: ""},
			"podman-compose",
		},
		{
			"podman compose",
			ComposeCommand{Binary: "podman", Subcommand: "compose"},
			"podman compose",
		},
		{
			"docker compose",
			ComposeCommand{Binary: "docker", Subcommand: "compose"},
			"docker compose",
		},
		{
			"docker-compose standalone",
			ComposeCommand{Binary: "docker-compose", Subcommand: ""},
			"docker-compose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cmd.String())
		})
	}
}

func TestNewComposeDetector(t *testing.T) {
	exec := &mockExecutor{}
	detector := NewComposeDetector(exec, logging.NopLogger{})
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.cache)
}

func TestNewComposeDetector_NilLogger(t *testing.T) {
	exec := &mockExecutor{}
	detector := NewComposeDetector(exec, nil)
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.logger)
}

func TestComposeDetector_Detect_PodmanComposePriority(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "test-host", Runtime: "podman"}

	callCount := 0
	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		callCount++
		if cmd == "podman-compose version --short" {
			return &CommandResult{ExitCode: 0, Stdout: "1.0.6"}, nil
		}
		return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result, err := detector.Detect(context.Background(), host)

	require.NoError(t, err)
	assert.Equal(t, "podman-compose", result.Name)
	assert.Equal(t, "podman-compose", result.Binary)
	assert.Equal(t, "", result.Subcommand)
	assert.Equal(t, "1.0.6", result.Version)
	assert.Equal(t, 1, callCount, "should stop at first successful detection")
}

func TestComposeDetector_Detect_DockerComposePriority(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "test-host", Runtime: "docker"}

	callCount := 0
	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		callCount++
		if cmd == "docker compose version --short" {
			return &CommandResult{ExitCode: 0, Stdout: "2.24.0"}, nil
		}
		return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result, err := detector.Detect(context.Background(), host)

	require.NoError(t, err)
	assert.Equal(t, "docker compose", result.Name)
	assert.Equal(t, "docker", result.Binary)
	assert.Equal(t, "compose", result.Subcommand)
	assert.Equal(t, "2.24.0", result.Version)
	assert.Equal(t, 2, callCount, "should try podman-compose first, then docker compose")
}

func TestComposeDetector_Detect_DockerComposeV1Fallback(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "test-host", Runtime: "docker"}

	callCount := 0
	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		callCount++
		if cmd == "docker-compose version --short" {
			return &CommandResult{ExitCode: 0, Stdout: "1.29.2"}, nil
		}
		return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result, err := detector.Detect(context.Background(), host)

	require.NoError(t, err)
	assert.Equal(t, "docker-compose", result.Name)
	assert.Equal(t, "docker-compose", result.Binary)
	assert.Equal(t, "", result.Subcommand)
	assert.Equal(t, "1.29.2", result.Version)
	assert.Equal(t, 4, callCount, "should try podman-compose, docker compose, podman compose, then docker-compose")
}

func TestComposeDetector_Detect_NoComposeFound(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "test-host", Runtime: ""}

	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		return &CommandResult{ExitCode: 1, Stderr: "command not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result, err := detector.Detect(context.Background(), host)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no compose command found")
	assert.Nil(t, result)
}

func TestComposeDetector_Detect_Cached(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "cached-host", Runtime: "podman"}

	callCount := 0
	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		callCount++
		return &CommandResult{ExitCode: 0, Stdout: "4.9.4"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})

	result1, err1 := detector.Detect(context.Background(), host)
	require.NoError(t, err1)
	assert.Equal(t, 1, callCount)

	result2, err2 := detector.Detect(context.Background(), host)
	require.NoError(t, err2)
	assert.Equal(t, 1, callCount, "should use cache on second call")

	assert.Equal(t, result1, result2)
}

func TestComposeDetector_Detect_ExecutorError(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "error-host", Runtime: ""}

	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		return nil, fmt.Errorf("connection refused")
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result, err := detector.Detect(context.Background(), host)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no compose command found")
	assert.Nil(t, result)
}

func TestComposeDetector_DetectWithFallback_Success(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "test-host", Runtime: "podman"}

	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		if cmd == "podman-compose version --short" {
			return &CommandResult{ExitCode: 0, Stdout: "1.0.6"}, nil
		}
		return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result := detector.DetectWithFallback(context.Background(), host)

	assert.NotNil(t, result)
	assert.Equal(t, "podman-compose", result.Name)
}

func TestComposeDetector_DetectWithFallback_UsesConfiguredRuntime(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "fallback-host", Runtime: "podman"}

	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result := detector.DetectWithFallback(context.Background(), host)

	assert.NotNil(t, result)
	assert.Equal(t, "podman compose", result.Name)
	assert.Equal(t, "podman", result.Binary)
}

func TestComposeDetector_DetectWithFallback_DefaultToDocker(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "no-runtime-host", Runtime: ""}

	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		return &CommandResult{ExitCode: 1, Stderr: "not found"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result := detector.DetectWithFallback(context.Background(), host)

	assert.NotNil(t, result)
	assert.Equal(t, "docker compose", result.Name)
	assert.Equal(t, "docker", result.Binary)
}

func TestComposeDetector_ClearCache_SingleHost(t *testing.T) {
	exec := &mockExecutor{}
	host1 := RemoteHost{Name: "host1", Runtime: "podman"}
	host2 := RemoteHost{Name: "host2", Runtime: "docker"}

	callCount := 0
	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		callCount++
		return &CommandResult{ExitCode: 0, Stdout: "4.9.4"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})

	_, _ = detector.Detect(context.Background(), host1)
	_, _ = detector.Detect(context.Background(), host2)
	assert.Equal(t, 2, callCount)

	detector.ClearCache("host1")

	_, _ = detector.Detect(context.Background(), host1)
	assert.Equal(t, 3, callCount, "should re-detect after clearing cache for host1")

	_, _ = detector.Detect(context.Background(), host2)
	assert.Equal(t, 3, callCount, "host2 cache should still be valid")
}

func TestComposeDetector_ClearCache_AllHosts(t *testing.T) {
	exec := &mockExecutor{}
	host1 := RemoteHost{Name: "host1", Runtime: "podman"}
	host2 := RemoteHost{Name: "host2", Runtime: "docker"}

	callCount := 0
	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		callCount++
		return &CommandResult{ExitCode: 0, Stdout: "4.9.4"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})

	_, _ = detector.Detect(context.Background(), host1)
	_, _ = detector.Detect(context.Background(), host2)
	assert.Equal(t, 2, callCount)

	detector.ClearCache("")

	_, _ = detector.Detect(context.Background(), host1)
	_, _ = detector.Detect(context.Background(), host2)
	assert.Equal(t, 4, callCount, "should re-detect both hosts after clearing all cache")
}

func TestKnownComposeCommands(t *testing.T) {
	commands := KnownComposeCommands()
	assert.Len(t, commands, 4)
	assert.Equal(t, "podman-compose", commands[0])
	assert.Equal(t, "docker compose", commands[1])
	assert.Equal(t, "podman compose", commands[2])
	assert.Equal(t, "docker-compose", commands[3])
}

func TestIsComposeCommand(t *testing.T) {
	assert.True(t, IsComposeCommand("podman-compose"))
	assert.True(t, IsComposeCommand("docker compose"))
	assert.True(t, IsComposeCommand("podman compose"))
	assert.True(t, IsComposeCommand("docker-compose"))
	assert.False(t, IsComposeCommand("kubectl"))
	assert.False(t, IsComposeCommand("nerdctl"))
	assert.False(t, IsComposeCommand("podman"))
}

func TestComposeDetector_ProbeCommand_VersionTrimmed(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{Name: "test-host", Runtime: "podman"}

	exec.executeFunc = func(ctx context.Context, h RemoteHost, cmd string) (*CommandResult, error) {
		return &CommandResult{ExitCode: 0, Stdout: "  4.9.4\n"}, nil
	}

	detector := NewComposeDetector(exec, logging.NopLogger{})
	result, err := detector.Detect(context.Background(), host)

	require.NoError(t, err)
	assert.Equal(t, "4.9.4", result.Version)
}
