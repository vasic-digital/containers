package remote

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
)

// unreachableHost returns a RemoteHost pointing at TEST-NET-1
// (RFC 5737) which is guaranteed to be unreachable.
func unreachableHost() RemoteHost {
	return RemoteHost{
		Name:    "unreachable",
		Address: "192.0.2.1",
		Port:    22,
		User:    "testuser",
		Auth:    AuthSSHKey,
	}
}

// newTestSSHExecutor creates an SSHExecutor with short timeouts
// and ControlMaster disabled so tests complete quickly.
func newTestSSHExecutor(t *testing.T) *SSHExecutor {
	t.Helper()
	exec, err := NewSSHExecutor(
		logging.NopLogger{},
		WithConnectTimeout(1*time.Second),
		WithCommandTimeout(2*time.Second),
		WithControlMaster(false),
	)
	require.NoError(t, err)
	return exec
}

func TestSSHExecutor_Execute_InvalidHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH executor test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name    string
		host    RemoteHost
		command string
	}{
		{
			name:    "unreachable host with simple command",
			host:    unreachableHost(),
			command: "echo hello",
		},
		{
			name: "unreachable host with different port",
			host: RemoteHost{
				Name:    "bad-port",
				Address: "192.0.2.1",
				Port:    2222,
				User:    "deploy",
				Auth:    AuthSSHKey,
			},
			command: "hostname",
		},
	}

	exec := newTestSSHExecutor(t)
	defer exec.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := exec.Execute(ctx, tc.host, tc.command)
			assert.Error(t, err,
				"Execute should fail for unreachable host")
		})
	}
}

func TestSSHExecutor_Execute_EmptyCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH executor test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name    string
		host    RemoteHost
		command string
	}{
		{
			name:    "empty command string",
			host:    unreachableHost(),
			command: "",
		},
		{
			name:    "whitespace-only command",
			host:    unreachableHost(),
			command: "   ",
		},
	}

	exec := newTestSSHExecutor(t)
	defer exec.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := exec.Execute(ctx, tc.host, tc.command)
			// With an unreachable host, any command (even empty)
			// should fail due to connection timeout.
			assert.Error(t, err,
				"Execute should fail for unreachable host "+
					"regardless of command content")
		})
	}
}

func TestSSHExecutor_CopyFile_InvalidHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH executor test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name       string
		host       RemoteHost
		localPath  string
		remotePath string
	}{
		{
			name:       "copy to unreachable host",
			host:       unreachableHost(),
			localPath:  "/tmp/nonexistent.txt",
			remotePath: "/remote/file.txt",
		},
		{
			name: "copy with custom port",
			host: RemoteHost{
				Name:    "bad-host",
				Address: "192.0.2.1",
				Port:    2222,
				User:    "deploy",
			},
			localPath:  "/tmp/test.txt",
			remotePath: "/remote/test.txt",
		},
	}

	exec := newTestSSHExecutor(t)
	defer exec.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancel()

			err := exec.CopyFile(
				ctx, tc.host, tc.localPath, tc.remotePath,
			)
			assert.Error(t, err,
				"CopyFile should fail for unreachable host")
			assert.Contains(t, err.Error(), "scp to")
		})
	}
}

func TestSSHExecutor_CopyDir_InvalidHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH executor test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name      string
		host      RemoteHost
		localDir  string
		remoteDir string
	}{
		{
			name:      "copy dir to unreachable host",
			host:      unreachableHost(),
			localDir:  "/tmp",
			remoteDir: "/remote/dir",
		},
		{
			name: "copy dir with key path",
			host: RemoteHost{
				Name:    "key-host",
				Address: "192.0.2.1",
				Port:    22,
				User:    "admin",
				KeyPath: "/nonexistent/key",
			},
			localDir:  "/tmp",
			remoteDir: "/remote/data",
		},
	}

	exec := newTestSSHExecutor(t)
	defer exec.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancel()

			err := exec.CopyDir(
				ctx, tc.host, tc.localDir, tc.remoteDir,
			)
			assert.Error(t, err,
				"CopyDir should fail for unreachable host")
			assert.Contains(t, err.Error(), "scp dir to")
		})
	}
}

func TestSSHExecutor_IsReachable_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH executor test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name string
		host RemoteHost
	}{
		{
			name: "TEST-NET-1 address",
			host: RemoteHost{
				Name:    "test-net-1",
				Address: "192.0.2.1",
				Port:    22,
				User:    "user",
			},
		},
		{
			name: "TEST-NET-1 with different port",
			host: RemoteHost{
				Name:    "test-net-1-alt",
				Address: "192.0.2.1",
				Port:    2222,
				User:    "deploy",
			},
		},
		{
			name: "TEST-NET-1 with key",
			host: RemoteHost{
				Name:    "test-net-1-key",
				Address: "192.0.2.1",
				Port:    22,
				User:    "admin",
				KeyPath: "/nonexistent/id_rsa",
			},
		},
	}

	exec := newTestSSHExecutor(t)
	defer exec.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			reachable := exec.IsReachable(ctx, tc.host)
			assert.False(t, reachable,
				"IsReachable should return false for "+
					"TEST-NET-1 (192.0.2.1)")
		})
	}
}

func TestSSHExecutor_ExecuteStream_InvalidHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSH executor test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name    string
		host    RemoteHost
		command string
	}{
		{
			name:    "stream on unreachable host",
			host:    unreachableHost(),
			command: "tail -f /var/log/syslog",
		},
		{
			name: "stream with custom user",
			host: RemoteHost{
				Name:    "stream-bad",
				Address: "192.0.2.1",
				Port:    22,
				User:    "root",
			},
			command: "journalctl -f",
		},
	}

	exec := newTestSSHExecutor(t)
	defer exec.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancel()

			reader, err := exec.ExecuteStream(
				ctx, tc.host, tc.command,
			)
			if err != nil {
				// Connection failed immediately -- expected.
				assert.Error(t, err)
				return
			}

			// If the stream started, reading should eventually
			// fail because the host is unreachable.
			defer reader.Close()
			buf := make([]byte, 1024)
			_, readErr := reader.Read(buf)
			assert.Error(t, readErr,
				"reading from stream on unreachable host "+
					"should fail")
		})
	}
}

func TestSSHExecutor_NewSSHExecutor_NilLogger(t *testing.T) {
	exec, err := NewSSHExecutor(
		nil,
		WithControlMaster(false),
	)
	require.NoError(t, err)
	defer exec.Close()
	assert.NotNil(t, exec)
	assert.NotNil(t, exec.logger,
		"nil logger should be replaced with NopLogger")
}

func TestSSHExecutor_Close_NoPool(t *testing.T) {
	exec, err := NewSSHExecutor(
		logging.NopLogger{},
		WithControlMaster(false),
	)
	require.NoError(t, err)

	err = exec.Close()
	assert.NoError(t, err,
		"Close without pool should succeed")
}
