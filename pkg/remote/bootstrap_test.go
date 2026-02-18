package remote

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"digital.vasic.containers/pkg/logging"
)

func TestBootstrapKeyAuth_NoPassword(t *testing.T) {
	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:    "test-host",
		Address: "10.0.0.1",
		User:    "user",
		KeyPath: "/tmp/fake-key",
	}

	err = exec.BootstrapKeyAuth(context.Background(), host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no password configured")
}

func TestBootstrapKeyAuth_NoKeyPath(t *testing.T) {
	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:     "test-host",
		Address:  "10.0.0.1",
		User:     "user",
		Password: "secret",
	}

	err = exec.BootstrapKeyAuth(context.Background(), host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no key path configured")
}

func TestBootstrapKeyAuth_MissingPubKey(t *testing.T) {
	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:     "test-host",
		Address:  "10.0.0.1",
		User:     "user",
		Password: "secret",
		KeyPath:  "/tmp/nonexistent-key-12345",
	}

	err = exec.BootstrapKeyAuth(context.Background(), host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read public key")
}

func TestBootstrapKeyAuth_EmptyPubKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_key")
	pubPath := keyPath + ".pub"

	// Create empty pub key file.
	require.NoError(t, os.WriteFile(pubPath, []byte(""), 0600))

	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:     "test-host",
		Address:  "10.0.0.1",
		User:     "user",
		Password: "secret",
		KeyPath:  keyPath,
	}

	err = exec.BootstrapKeyAuth(context.Background(), host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "public key")
	assert.Contains(t, err.Error(), "is empty")
}

func TestNeedsBootstrap_NoPassword(t *testing.T) {
	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:    "test-host",
		Address: "10.0.0.1",
		User:    "user",
		KeyPath: "/tmp/fake-key",
	}

	assert.False(t, exec.NeedsBootstrap(
		context.Background(), host,
	))
}

func TestNeedsBootstrap_NoKeyPath(t *testing.T) {
	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:     "test-host",
		Address:  "10.0.0.1",
		User:     "user",
		Password: "secret",
	}

	assert.False(t, exec.NeedsBootstrap(
		context.Background(), host,
	))
}

func TestNeedsBootstrap_KeyFileDoesNotExist(t *testing.T) {
	exec, err := NewSSHExecutor(logging.NopLogger{})
	require.NoError(t, err)
	defer exec.Close()

	host := RemoteHost{
		Name:     "test-host",
		Address:  "10.0.0.1",
		User:     "user",
		Password: "secret",
		KeyPath:  "/tmp/nonexistent-key-99999",
	}

	assert.False(t, exec.NeedsBootstrap(
		context.Background(), host,
	))
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"tilde prefix",
			"~/.ssh/id_ed25519",
			filepath.Join(home, ".ssh/id_ed25519"),
		},
		{
			"absolute path",
			"/home/user/.ssh/id_rsa",
			"/home/user/.ssh/id_rsa",
		},
		{
			"relative path",
			"keys/my_key",
			"keys/my_key",
		},
		{
			"tilde only",
			"~",
			home,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, expandHome(tt.input))
		})
	}
}

func TestSSHDial_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	config := &ssh.ClientConfig{
		User:            "test",
		Auth:            []ssh.AuthMethod{ssh.Password("pass")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	_, err := sshDial(ctx, "tcp", "127.0.0.1:22", config)
	assert.Error(t, err)
}
