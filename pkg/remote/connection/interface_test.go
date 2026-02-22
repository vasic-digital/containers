package connection

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteOptions(t *testing.T) {
	cfg := &ExecuteConfig{}
	WithTimeout(30)(cfg)
	assert.Equal(t, 30, cfg.Timeout)

	WithEnv(map[string]string{"KEY": "value"})(cfg)
	assert.Equal(t, "value", cfg.Env["KEY"])

	WithWorkingDir("/tmp")(cfg)
	assert.Equal(t, "/tmp", cfg.WorkingDir)

	WithUser("testuser")(cfg)
	assert.Equal(t, "testuser", cfg.User)

	WithCaptureStderr(true)(cfg)
	assert.True(t, cfg.CaptureStderr)
}

func TestTransferOptions(t *testing.T) {
	cfg := &TransferConfig{}
	WithTransferTimeout(60)(cfg)
	assert.Equal(t, 60, cfg.Timeout)

	WithPermissions("0644")(cfg)
	assert.Equal(t, "0644", cfg.Permissions)

	WithOverwrite(true)(cfg)
	assert.True(t, cfg.Overwrite)

	called := false
	WithProgress(func(_, _ int64) { called = true })(cfg)
	cfg.Progress(0, 0)
	assert.True(t, called)
}

func TestShellOptions(t *testing.T) {
	cfg := &ShellConfig{}
	WithTerminal("xterm-256color")(cfg)
	assert.Equal(t, "xterm-256color", cfg.Terminal)

	WithShellEnv(map[string]string{"TERM": "xterm"})(cfg)
	assert.Equal(t, "xterm", cfg.Env["TERM"])

	WithTerminalSize(40, 120)(cfg)
	assert.Equal(t, uint16(40), cfg.Rows)
	assert.Equal(t, uint16(120), cfg.Cols)
}

func TestLogOptions(t *testing.T) {
	cfg := &LogConfig{}
	WithFollow(true)(cfg)
	assert.True(t, cfg.Follow)

	WithSince("1h")(cfg)
	assert.Equal(t, "1h", cfg.Since)

	WithUntil("now")(cfg)
	assert.Equal(t, "now", cfg.Until)

	WithTail("100")(cfg)
	assert.Equal(t, "100", cfg.Tail)

	WithTimestamps(true)(cfg)
	assert.True(t, cfg.Timestamps)
}

func TestCloudProvider_Values(t *testing.T) {
	providers := []CloudProvider{
		ProviderAWS,
		ProviderAzure,
		ProviderGCP,
	}
	for _, p := range providers {
		assert.NotEmpty(t, string(p))
	}
}

func TestContainerFilter(t *testing.T) {
	filter := ContainerFilter{
		All:    true,
		Labels: map[string]string{"app": "nginx"},
		Names:  []string{"web"},
		Status: []string{"running"},
	}
	assert.True(t, filter.All)
	assert.Equal(t, "nginx", filter.Labels["app"])
}

func TestContainerInfo(t *testing.T) {
	info := ContainerInfo{
		ID:      "abc123",
		Name:    "web",
		Image:   "nginx:latest",
		State:   "running",
		Status:  "Up 2 hours",
		Labels:  map[string]string{"app": "nginx"},
		Created: 1234567890,
	}
	assert.Equal(t, "abc123", info.ID)
	assert.Equal(t, "web", info.Name)
}

func TestExecuteConfig_Defaults(t *testing.T) {
	cfg := &ExecuteConfig{}
	assert.Equal(t, 0, cfg.Timeout)
	assert.Nil(t, cfg.Env)
	assert.Empty(t, cfg.WorkingDir)
}

func TestTransferConfig_Defaults(t *testing.T) {
	cfg := &TransferConfig{}
	assert.Equal(t, 0, cfg.Timeout)
	assert.Empty(t, cfg.Permissions)
	assert.False(t, cfg.Overwrite)
	assert.Nil(t, cfg.Progress)
}

func TestShellConfig_Defaults(t *testing.T) {
	cfg := &ShellConfig{}
	assert.Empty(t, cfg.Terminal)
	assert.Nil(t, cfg.Env)
	assert.Equal(t, uint16(0), cfg.Rows)
	assert.Equal(t, uint16(0), cfg.Cols)
}

func TestLogConfig_Defaults(t *testing.T) {
	cfg := &LogConfig{}
	assert.False(t, cfg.Follow)
	assert.Empty(t, cfg.Since)
	assert.Empty(t, cfg.Until)
	assert.Empty(t, cfg.Tail)
	assert.False(t, cfg.Timestamps)
}
