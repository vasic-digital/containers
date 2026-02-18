package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteHost_SSHPort(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		expected int
	}{
		{"default when zero", 0, 22},
		{"default when negative", -1, 22},
		{"custom port", 2222, 2222},
		{"standard port", 22, 22},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := RemoteHost{Port: tt.port}
			assert.Equal(t, tt.expected, host.SSHPort())
		})
	}
}

func TestHostResources_AvailableMemoryPercent(t *testing.T) {
	tests := []struct {
		name     string
		used     float64
		expected float64
	}{
		{"50% used", 50.0, 50.0},
		{"0% used", 0.0, 100.0},
		{"100% used", 100.0, 0.0},
		{"75.5% used", 75.5, 24.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HostResources{MemoryPercent: tt.used}
			assert.InDelta(t,
				tt.expected, r.AvailableMemoryPercent(), 0.01,
			)
		})
	}
}

func TestHostResources_AvailableDiskPercent(t *testing.T) {
	r := &HostResources{DiskPercent: 60.0}
	assert.InDelta(t, 40.0, r.AvailableDiskPercent(), 0.01)
}

func TestHostResources_AvailableCPUPercent(t *testing.T) {
	r := &HostResources{CPUPercent: 30.0}
	assert.InDelta(t, 70.0, r.AvailableCPUPercent(), 0.01)
}

func TestAuthMethod_Values(t *testing.T) {
	assert.Equal(t, AuthMethod("ssh_key"), AuthSSHKey)
	assert.Equal(t, AuthMethod("ssh_agent"), AuthSSHAgent)
	assert.Equal(t, AuthMethod("password"), AuthPassword)
}

func TestHostState_Values(t *testing.T) {
	assert.Equal(t, HostState("online"), HostOnline)
	assert.Equal(t, HostState("offline"), HostOffline)
	assert.Equal(t, HostState("degraded"), HostDegraded)
	assert.Equal(t, HostState("unknown"), HostUnknown)
}

func TestCommandResult_Fields(t *testing.T) {
	r := &CommandResult{
		Stdout:   "hello\n",
		Stderr:   "",
		ExitCode: 0,
		Duration: 100 * time.Millisecond,
	}
	require.Equal(t, "hello\n", r.Stdout)
	require.Equal(t, 0, r.ExitCode)
	require.Equal(t, 100*time.Millisecond, r.Duration)
}

func TestRemoteHost_FullConfig(t *testing.T) {
	host := RemoteHost{
		Name:          "test-server",
		Address:       "192.168.1.100",
		Port:          2222,
		User:          "deploy",
		KeyPath:       "/home/user/.ssh/id_rsa",
		Auth:          AuthSSHKey,
		Runtime:       "docker",
		Labels:        map[string]string{"gpu": "true"},
		MaxContainers: 10,
	}

	assert.Equal(t, "test-server", host.Name)
	assert.Equal(t, 2222, host.SSHPort())
	assert.Equal(t, "docker", host.Runtime)
	assert.Equal(t, "true", host.Labels["gpu"])
	assert.Equal(t, 10, host.MaxContainers)
}
