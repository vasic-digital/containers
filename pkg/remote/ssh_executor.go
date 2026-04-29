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
//
// Semantics: after this call, `remoteDir` on the remote host has the
// same contents as `localDir` on the local host. This is NOT what
// `scp -r local remote` natively gives you — when `remote` already
// exists, scp nests (creates `remote/<basename(local)>`). To deliver
// the rsync-style semantics callers expect, the function ensures the
// remote destination is removed before the scp runs. The remote PARENT
// dir is created if missing so the subsequent `scp -r` lands in the
// right place.
//
// Forensic note (2026-04-29): without the pre-clean step, when a
// caller copied a sibling file (e.g. Dockerfile) into the parent of
// the destination first — which created the destination as a
// side-effect of `mkdir -p <remoteParent>` — the directory copy then
// nested INSIDE that pre-existing destination. Cognee build broke
// remotely because `external/cognee/README.md` ended up at
// `external/cognee/cognee/README.md` (one level too deep), and the
// COPY directive in the Dockerfile failed with
// `copier: stat: "/README.md": no such file or directory`. The
// pre-clean step removes that ambiguity entirely: scp always creates
// the destination, so the result is deterministic regardless of which
// caller ran first.
func (e *SSHExecutor) CopyDir(
	ctx context.Context,
	host RemoteHost,
	localDir, remoteDir string,
) error {
	// Defensive cleanup: remove the destination if it exists so scp -r
	// creates it freshly rather than nesting inside it. Use a single
	// SSH round-trip and ignore non-zero exits (the most common cause
	// is "directory does not exist", which is the desired pre-state
	// anyway).
	cleanCmd := fmt.Sprintf("rm -rf -- %s", shellEscape(remoteDir))
	if sshArgsList, sshErr := e.sshArgs(ctx, host); sshErr == nil {
		cleanArgs := append(sshArgsList, fmt.Sprintf("%s@%s", host.User, host.Address), cleanCmd)
		cleanCmdRun := exec.CommandContext(ctx, "ssh", cleanArgs...)
		var cleanStderr bytes.Buffer
		cleanCmdRun.Stderr = &cleanStderr
		// Best-effort: log but don't fail. A non-existent remoteDir
		// produces a benign `rm: cannot remove ...: No such file or
		// directory` we explicitly want to ignore.
		if err := cleanCmdRun.Run(); err != nil {
			e.logger.Debug(
				"pre-scp rm on %s (ignored): %s -> %s: %v (stderr: %s)",
				host.Name, localDir, remoteDir, err, cleanStderr.String(),
			)
		}
	}

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

// shellEscape wraps an arbitrary path in single quotes for safe inclusion
// in a remote shell command, doubling any embedded single quotes.
func shellEscape(s string) string {
	// 'foo' -> 'foo'; foo's -> 'foo'\''s'
	if !needsShellEscape(s) {
		return s
	}
	const single = `'`
	const escape = `'\''`
	return single + replaceAll(s, single, escape) + single
}

func needsShellEscape(s string) bool {
	for _, r := range s {
		// Allow [A-Za-z0-9_./-] without quoting; everything else
		// (including spaces, $, ", and special chars) gets quoted.
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '/' || r == '.' || r == '-':
		default:
			return true
		}
	}
	return false
}

func replaceAll(s, old, new string) string {
	if old == new || old == "" {
		return s
	}
	out := ""
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out += new
			i += len(old)
		} else {
			out += string(s[i])
			i++
		}
	}
	return out
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
	}

	// Keep-alive probes: send every KeepAlive seconds; drop the
	// session after KeepAliveCountMax missed probes. This keeps
	// the TCP channel alive during long-running remote operations
	// (e.g. multi-minute image builds over compose up).
	if e.opts.KeepAlive > 0 && e.opts.KeepAliveCountMax > 0 {
		args = append(args,
			"-o", fmt.Sprintf(
				"ServerAliveInterval=%d",
				int(e.opts.KeepAlive.Seconds()),
			),
			"-o", fmt.Sprintf(
				"ServerAliveCountMax=%d",
				e.opts.KeepAliveCountMax,
			),
		)
	}

	args = append(args, "-p", strconv.Itoa(host.SSHPort()))

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
	}

	// Keep scp transfers alive across network blips (same
	// rationale as sshArgs).
	if e.opts.KeepAlive > 0 && e.opts.KeepAliveCountMax > 0 {
		args = append(args,
			"-o", fmt.Sprintf(
				"ServerAliveInterval=%d",
				int(e.opts.KeepAlive.Seconds()),
			),
			"-o", fmt.Sprintf(
				"ServerAliveCountMax=%d",
				e.opts.KeepAliveCountMax,
			),
		)
	}

	args = append(args, "-P", strconv.Itoa(host.SSHPort()))

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
