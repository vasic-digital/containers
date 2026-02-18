package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"digital.vasic.containers/pkg/logging"
)

// SSHExecutor implements RemoteExecutor using the ssh/scp CLI tools
// with optional ControlMaster connection pooling.
type SSHExecutor struct {
	opts   Options
	pool   *ConnectionPool
	logger logging.Logger
}

// NewSSHExecutor creates an SSHExecutor with the given options.
func NewSSHExecutor(
	logger logging.Logger, opts ...Option,
) (*SSHExecutor, error) {
	o := ApplyOptions(opts)
	if logger == nil {
		logger = logging.NopLogger{}
	}

	var pool *ConnectionPool
	if o.ControlMasterEnabled {
		var err error
		pool, err = NewConnectionPool(o)
		if err != nil {
			return nil, fmt.Errorf(
				"create connection pool: %w", err,
			)
		}
	}

	return &SSHExecutor{
		opts:   o,
		pool:   pool,
		logger: logger,
	}, nil
}

// Execute runs a command on the remote host and returns its output.
func (e *SSHExecutor) Execute(
	ctx context.Context,
	host RemoteHost,
	command string,
) (*CommandResult, error) {
	start := time.Now()

	if e.opts.CommandTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.CommandTimeout)
		defer cancel()
	}

	args, err := e.sshArgs(ctx, host)
	if err != nil {
		return nil, err
	}
	args = append(args, command)

	e.logger.Debug("ssh exec on %s: %s", host.Name, command)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf(
				"ssh exec on %s exited with code %d: %s",
				host.Name, result.ExitCode, stderr.String(),
			)
		}
		return result, fmt.Errorf(
			"ssh exec on %s: %w", host.Name, runErr,
		)
	}

	if e.pool != nil {
		e.pool.Release(host)
	}

	return result, nil
}

// ExecuteStream runs a command on the remote host and returns a
// reader for streaming its output.
func (e *SSHExecutor) ExecuteStream(
	ctx context.Context,
	host RemoteHost,
	command string,
) (io.ReadCloser, error) {
	args, err := e.sshArgs(ctx, host)
	if err != nil {
		return nil, err
	}
	args = append(args, command)

	e.logger.Debug(
		"ssh stream on %s: %s", host.Name, command,
	)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"ssh stream start on %s: %w", host.Name, err,
		)
	}

	return &streamReader{
		cmd:    cmd,
		reader: stdout,
		pool:   e.pool,
		host:   host,
	}, nil
}

// CopyFile copies a local file to a remote host using scp.
func (e *SSHExecutor) CopyFile(
	ctx context.Context,
	host RemoteHost,
	localPath, remotePath string,
) error {
	args := e.scpArgs(host)
	args = append(args,
		localPath,
		fmt.Sprintf(
			"%s@%s:%s", host.User, host.Address, remotePath,
		),
	)

	e.logger.Debug(
		"scp file to %s: %s -> %s",
		host.Name, localPath, remotePath,
	)

	cmd := exec.CommandContext(ctx, "scp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"scp to %s: %w (stderr: %s)",
			host.Name, err, stderr.String(),
		)
	}
	return nil
}

// CopyDir copies a local directory to a remote host using scp -r.
func (e *SSHExecutor) CopyDir(
	ctx context.Context,
	host RemoteHost,
	localDir, remoteDir string,
) error {
	args := e.scpArgs(host)
	args = append([]string{"-r"}, args...)
	args = append(args,
		localDir,
		fmt.Sprintf(
			"%s@%s:%s", host.User, host.Address, remoteDir,
		),
	)

	e.logger.Debug(
		"scp dir to %s: %s -> %s",
		host.Name, localDir, remoteDir,
	)

	cmd := exec.CommandContext(ctx, "scp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"scp dir to %s: %w (stderr: %s)",
			host.Name, err, stderr.String(),
		)
	}
	return nil
}

// IsReachable checks whether the host accepts SSH connections.
func (e *SSHExecutor) IsReachable(
	ctx context.Context, host RemoteHost,
) bool {
	timeout := e.opts.ConnectTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := e.Execute(ctx, host, "echo ok")
	if err != nil {
		return false
	}
	return strings.TrimSpace(result.Stdout) == "ok"
}

// Close releases any resources held by the executor.
func (e *SSHExecutor) Close() error {
	if e.pool != nil {
		return e.pool.Close()
	}
	return nil
}

func (e *SSHExecutor) sshArgs(
	ctx context.Context, host RemoteHost,
) ([]string, error) {
	args := []string{
		"-o", "StrictHostKeyChecking=" +
			boolToYesNo(e.opts.StrictHostKeyCheck),
		"-o", fmt.Sprintf(
			"ConnectTimeout=%d",
			int(e.opts.ConnectTimeout.Seconds()),
		),
		"-o", "BatchMode=yes",
		"-p", strconv.Itoa(host.SSHPort()),
	}

	if e.pool != nil && e.opts.ControlMasterEnabled {
		socketPath, err := e.pool.Acquire(ctx, host)
		if err != nil {
			e.logger.Warn(
				"ControlMaster unavailable for %s, "+
					"falling back to direct: %v",
				host.Name, err,
			)
		} else {
			args = append(args, "-S", socketPath)
		}
	}

	if host.KeyPath != "" {
		args = append(args, "-i", host.KeyPath)
	}

	args = append(args,
		fmt.Sprintf("%s@%s", host.User, host.Address),
	)
	return args, nil
}

func (e *SSHExecutor) scpArgs(host RemoteHost) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=" +
			boolToYesNo(e.opts.StrictHostKeyCheck),
		"-o", fmt.Sprintf(
			"ConnectTimeout=%d",
			int(e.opts.ConnectTimeout.Seconds()),
		),
		"-o", "BatchMode=yes",
		"-P", strconv.Itoa(host.SSHPort()),
	}

	if host.KeyPath != "" {
		args = append(args, "-i", host.KeyPath)
	}

	return args
}

// streamReader wraps an SSH command's stdout pipe.
type streamReader struct {
	cmd    *exec.Cmd
	reader io.ReadCloser
	pool   *ConnectionPool
	host   RemoteHost
}

func (r *streamReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *streamReader) Close() error {
	_ = r.reader.Close()
	err := r.cmd.Wait()
	if r.pool != nil {
		r.pool.Release(r.host)
	}
	return err
}
