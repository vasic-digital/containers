package envconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/remote"
)

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables.
	t.Setenv("CONTAINERS_REMOTE_ENABLED", "true")
	t.Setenv("CONTAINERS_REMOTE_DEFAULT_SSH_USER", "deploy")
	t.Setenv("CONTAINERS_REMOTE_DEFAULT_SSH_KEY", "/home/user/.ssh/id_rsa")
	t.Setenv("CONTAINERS_REMOTE_DEFAULT_RUNTIME", "podman")
	t.Setenv("CONTAINERS_REMOTE_SCHEDULER", "round_robin")
	t.Setenv("CONTAINERS_REMOTE_PORT_RANGE_START", "25000")
	t.Setenv("CONTAINERS_REMOTE_PORT_RANGE_END", "35000")
	t.Setenv("CONTAINERS_REMOTE_VOLUME_TYPE", "nfs")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_NAME", "gpu-1")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_ADDRESS", "10.0.0.1")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_PORT", "2222")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_USER", "admin")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_KEY", "/keys/gpu")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_RUNTIME", "docker")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_LABELS", "gpu=true,arch=amd64")

	cfg := LoadFromEnv()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "deploy", cfg.DefaultUser)
	assert.Equal(t, "/home/user/.ssh/id_rsa", cfg.DefaultKeyPath)
	assert.Equal(t, "podman", cfg.DefaultRuntime)
	assert.Equal(t, "round_robin", cfg.Scheduler)
	assert.Equal(t, 25000, cfg.PortRangeStart)
	assert.Equal(t, 35000, cfg.PortRangeEnd)
	assert.Equal(t, "nfs", cfg.VolumeType)

	require.Len(t, cfg.Hosts, 1)
	assert.Equal(t, "gpu-1", cfg.Hosts[0].Name)
	assert.Equal(t, "10.0.0.1", cfg.Hosts[0].Address)
	assert.Equal(t, 2222, cfg.Hosts[0].Port)
	assert.Equal(t, "admin", cfg.Hosts[0].User)
	assert.Equal(t, "docker", cfg.Hosts[0].Runtime)
	assert.Equal(t, "true", cfg.Hosts[0].Labels["gpu"])
	assert.Equal(t, "amd64", cfg.Hosts[0].Labels["arch"])
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Clear relevant env vars.
	os.Unsetenv("CONTAINERS_REMOTE_ENABLED")
	os.Unsetenv("CONTAINERS_REMOTE_DEFAULT_RUNTIME")

	cfg := LoadFromEnv()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "docker", cfg.DefaultRuntime)
	assert.Equal(t, "resource_aware", cfg.Scheduler)
	assert.Equal(t, 20000, cfg.PortRangeStart)
	assert.Equal(t, 30000, cfg.PortRangeEnd)
	assert.Equal(t, "sshfs", cfg.VolumeType)
}

func TestLoadFromEnv_MultipleHosts(t *testing.T) {
	t.Setenv("CONTAINERS_REMOTE_HOST_1_NAME", "host-1")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_ADDRESS", "10.0.0.1")
	t.Setenv("CONTAINERS_REMOTE_HOST_2_NAME", "host-2")
	t.Setenv("CONTAINERS_REMOTE_HOST_2_ADDRESS", "10.0.0.2")
	t.Setenv("CONTAINERS_REMOTE_HOST_3_NAME", "host-3")
	t.Setenv("CONTAINERS_REMOTE_HOST_3_ADDRESS", "10.0.0.3")

	cfg := LoadFromEnv()
	assert.Len(t, cfg.Hosts, 3)
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{"empty", "", nil},
		{
			"single",
			"gpu=true",
			map[string]string{"gpu": "true"},
		},
		{
			"multiple",
			"gpu=true,arch=amd64,env=prod",
			map[string]string{
				"gpu": "true", "arch": "amd64", "env": "prod",
			},
		},
		{
			"with spaces",
			"gpu = true , arch = amd64",
			map[string]string{"gpu": "true", "arch": "amd64"},
		},
		{
			"trailing comma",
			"gpu=true,",
			map[string]string{"gpu": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLabels(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToRemoteHosts(t *testing.T) {
	cfg := &DistributionConfig{
		DefaultUser:    "deploy",
		DefaultKeyPath: "/default/key",
		DefaultRuntime: "docker",
		Hosts: []RemoteEndpointConfig{
			{
				Name:    "host-1",
				Address: "10.0.0.1",
				Port:    2222,
				User:    "admin",
				Labels:  map[string]string{"gpu": "true"},
			},
			{
				Name:    "host-2",
				Address: "10.0.0.2",
				// Uses defaults for user, key, runtime, port.
			},
		},
	}

	hosts := cfg.ToRemoteHosts()
	require.Len(t, hosts, 2)

	assert.Equal(t, "admin", hosts[0].User)
	assert.Equal(t, 2222, hosts[0].Port)

	assert.Equal(t, "deploy", hosts[1].User)
	assert.Equal(t, "/default/key", hosts[1].KeyPath)
	assert.Equal(t, "docker", hosts[1].Runtime)
	assert.Equal(t, 22, hosts[1].Port)
}

func TestEnvString(t *testing.T) {
	t.Setenv("TEST_ENVCONFIG_STRING", "hello")
	assert.Equal(t, "hello", envString("TEST_ENVCONFIG_STRING", "default"))
	assert.Equal(t, "default", envString("TEST_ENVCONFIG_MISSING", "default"))
}

func TestEnvInt(t *testing.T) {
	t.Setenv("TEST_ENVCONFIG_INT", "42")
	assert.Equal(t, 42, envInt("TEST_ENVCONFIG_INT", 0))
	assert.Equal(t, 99, envInt("TEST_ENVCONFIG_MISSING", 99))
}

func TestEnvBool(t *testing.T) {
	t.Setenv("TEST_ENVCONFIG_BOOL", "true")
	assert.True(t, envBool("TEST_ENVCONFIG_BOOL", false))
	assert.False(t, envBool("TEST_ENVCONFIG_MISSING", false))
}

func TestLoadFromEnv_PasswordAndSSHOptions(t *testing.T) {
	t.Setenv("CONTAINERS_REMOTE_ENABLED", "true")
	t.Setenv("CONTAINERS_REMOTE_DEFAULT_SSH_USER", "deploy")
	t.Setenv("CONTAINERS_REMOTE_DEFAULT_SSH_KEY", "~/.ssh/id_ed25519")
	t.Setenv("CONTAINERS_REMOTE_DEFAULT_SSH_PASSWORD", "secret123")
	t.Setenv("CONTAINERS_REMOTE_CONNECT_TIMEOUT", "15")
	t.Setenv("CONTAINERS_REMOTE_COMMAND_TIMEOUT", "180")
	t.Setenv("CONTAINERS_REMOTE_SSH_CONTROL_MASTER", "false")
	t.Setenv("CONTAINERS_REMOTE_SSH_CONTROL_PERSIST", "600")
	t.Setenv("CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS", "20")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_NAME", "srv-1")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_ADDRESS", "10.0.0.1")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_PASSWORD", "host-pass")

	cfg := LoadFromEnv()

	assert.Equal(t, "secret123", cfg.DefaultPassword)
	assert.Equal(t, 15, cfg.ConnectTimeout)
	assert.Equal(t, 180, cfg.CommandTimeout)
	assert.False(t, cfg.ControlMasterEnabled)
	assert.Equal(t, 600, cfg.ControlPersist)
	assert.Equal(t, 20, cfg.MaxConnections)

	require.Len(t, cfg.Hosts, 1)
	assert.Equal(t, "host-pass", cfg.Hosts[0].Password)
}

func TestToRemoteHosts_PasswordAuth(t *testing.T) {
	cfg := &DistributionConfig{
		DefaultUser:     "deploy",
		DefaultKeyPath:  "/default/key",
		DefaultPassword: "default-pass",
		DefaultRuntime:  "docker",
		Hosts: []RemoteEndpointConfig{
			{
				Name:     "with-password",
				Address:  "10.0.0.1",
				Password: "host-pass",
			},
			{
				Name:    "inherits-password",
				Address: "10.0.0.2",
			},
			{
				Name:    "no-password",
				Address: "10.0.0.3",
			},
		},
	}

	hosts := cfg.ToRemoteHosts()
	require.Len(t, hosts, 3)

	// Host with explicit password.
	assert.Equal(t, "host-pass", hosts[0].Password)
	assert.Equal(t, remote.AuthPassword, hosts[0].Auth)

	// Host inheriting default password.
	assert.Equal(t, "default-pass", hosts[1].Password)
	assert.Equal(t, remote.AuthPassword, hosts[1].Auth)

	// Host inheriting default password.
	assert.Equal(t, "default-pass", hosts[2].Password)
	assert.Equal(t, remote.AuthPassword, hosts[2].Auth)
}

func TestLoadFromEnv_SSHOptionDefaults(t *testing.T) {
	// Clear relevant env vars to test defaults.
	os.Unsetenv("CONTAINERS_REMOTE_CONNECT_TIMEOUT")
	os.Unsetenv("CONTAINERS_REMOTE_COMMAND_TIMEOUT")
	os.Unsetenv("CONTAINERS_REMOTE_SSH_CONTROL_MASTER")
	os.Unsetenv("CONTAINERS_REMOTE_SSH_CONTROL_PERSIST")
	os.Unsetenv("CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS")

	cfg := LoadFromEnv()

	assert.Equal(t, 10, cfg.ConnectTimeout)
	assert.Equal(t, 120, cfg.CommandTimeout)
	assert.True(t, cfg.ControlMasterEnabled)
	assert.Equal(t, 300, cfg.ControlPersist)
	assert.Equal(t, 10, cfg.MaxConnections)
}

func TestParse_HostGPUAutoprobe(t *testing.T) {
	t.Setenv("CONTAINERS_REMOTE_ENABLED", "true")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_NAME", "thinker")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_ADDRESS", "thinker.local")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_USER", "milosvasic")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_LABELS", "gpu=true,cuda=12.2")
	t.Setenv("CONTAINERS_REMOTE_HOST_1_GPU_AUTOPROBE", "true")

	cfg, err := Parse()
	require.NoError(t, err)
	require.Len(t, cfg.Hosts, 1)
	h := cfg.Hosts[0]
	require.Equal(t, "true", h.Labels["gpu_autoprobe"])
	require.Equal(t, "12.2", h.Labels["cuda"])
}
