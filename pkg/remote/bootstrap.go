package remote

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// BootstrapKeyAuth connects to the remote host using password
// authentication (Go-native SSH), reads the local public key, and
// appends it to the remote ~/.ssh/authorized_keys. After bootstrap,
// key-based auth via the CLI SSH executor should work.
func (e *SSHExecutor) BootstrapKeyAuth(
	ctx context.Context, host RemoteHost,
) error {
	if host.Password == "" {
		return fmt.Errorf(
			"bootstrap %s: no password configured", host.Name,
		)
	}
	if host.KeyPath == "" {
		return fmt.Errorf(
			"bootstrap %s: no key path configured", host.Name,
		)
	}

	pubKeyPath := host.KeyPath + ".pub"
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf(
			"read public key %s: %w", pubKeyPath, err,
		)
	}
	pubKey := strings.TrimSpace(string(pubKeyBytes))
	if pubKey == "" {
		return fmt.Errorf(
			"public key %s is empty", pubKeyPath,
		)
	}

	e.logger.Info(
		"bootstrapping key auth on %s (%s@%s:%d)",
		host.Name, host.User, host.Address, host.SSHPort(),
	)

	timeout := e.opts.ConnectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	config := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(host.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := net.JoinHostPort(
		host.Address, fmt.Sprintf("%d", host.SSHPort()),
	)

	client, err := sshDial(ctx, "tcp", addr, config)
	if err != nil {
		return fmt.Errorf(
			"bootstrap SSH connect to %s: %w", host.Name, err,
		)
	}
	defer client.Close()

	// Create ~/.ssh directory and append the public key.
	script := fmt.Sprintf(
		`mkdir -p ~/.ssh && chmod 700 ~/.ssh && `+
			`touch ~/.ssh/authorized_keys && `+
			`chmod 600 ~/.ssh/authorized_keys && `+
			`if ! grep -qF %q ~/.ssh/authorized_keys; then `+
			`echo %q >> ~/.ssh/authorized_keys; fi`,
		pubKey, pubKey,
	)

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf(
			"bootstrap session on %s: %w", host.Name, err,
		)
	}
	defer session.Close()

	output, err := session.CombinedOutput(script)
	if err != nil {
		return fmt.Errorf(
			"bootstrap script on %s: %w (output: %s)",
			host.Name, err, string(output),
		)
	}

	e.logger.Info(
		"public key appended on %s, verifying key auth",
		host.Name,
	)

	// Verify key auth works via the normal CLI SSH path.
	verifyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := e.Execute(verifyCtx, host, "echo ok")
	if err != nil {
		return fmt.Errorf(
			"key auth verification on %s failed: %w",
			host.Name, err,
		)
	}
	if strings.TrimSpace(result.Stdout) != "ok" {
		return fmt.Errorf(
			"key auth verification on %s: unexpected output %q",
			host.Name, result.Stdout,
		)
	}

	e.logger.Info(
		"key auth bootstrap complete on %s", host.Name,
	)
	return nil
}

// NeedsBootstrap checks whether key-based SSH authentication works
// for the given host. Returns true when the host has a password
// configured and key auth fails.
func (e *SSHExecutor) NeedsBootstrap(
	ctx context.Context, host RemoteHost,
) bool {
	if host.Password == "" || host.KeyPath == "" {
		return false
	}
	// Expand ~ in key path.
	keyPath := expandHome(host.KeyPath)
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	timeout := e.opts.ConnectTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return !e.IsReachable(checkCtx, host)
}

// sshDial connects to an SSH server, respecting context cancellation.
func sshDial(
	ctx context.Context,
	network, addr string,
	config *ssh.ClientConfig,
) (*ssh.Client, error) {
	dialer := net.Dialer{Timeout: config.Timeout}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
