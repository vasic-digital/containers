package remote

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/logging"
)

// RemoteComposeOrchestrator implements compose.ComposeOrchestrator
// by executing compose commands on a remote host via SSH.
//
// The orchestrator uses intelligent compose command detection with the
// following priority order:
//  1. podman compose (preferred for rootless, systemd integration)
//  2. docker compose (v2, plugin-based)
//  3. docker-compose (v1, standalone)
//
// If detection fails, it falls back to the host's configured runtime.
type RemoteComposeOrchestrator struct {
	host       RemoteHost
	executor   RemoteExecutor
	composeCmd *ComposeCommand
	detector   *ComposeDetector
	logger     logging.Logger
	once       sync.Once
	detectErr  error
}

// RemoteComposeOption configures the RemoteComposeOrchestrator.
type RemoteComposeOption func(*RemoteComposeOrchestrator)

// WithComposeCommand forces a specific compose command instead of auto-detection.
func WithComposeCommand(cmd string) RemoteComposeOption {
	return func(o *RemoteComposeOrchestrator) {
		parts := strings.SplitN(cmd, " ", 2)
		o.composeCmd = &ComposeCommand{
			Name:   cmd,
			Binary: parts[0],
		}
		if len(parts) > 1 {
			o.composeCmd.Subcommand = parts[1]
		}
	}
}

// WithComposeDetector sets a custom compose detector.
func WithComposeDetector(detector *ComposeDetector) RemoteComposeOption {
	return func(o *RemoteComposeOrchestrator) {
		o.detector = detector
	}
}

// NewRemoteComposeOrchestrator creates a compose orchestrator that
// operates on a remote host with intelligent compose command detection.
func NewRemoteComposeOrchestrator(
	host RemoteHost,
	executor RemoteExecutor,
	logger logging.Logger,
	opts ...RemoteComposeOption,
) *RemoteComposeOrchestrator {
	if logger == nil {
		logger = logging.NopLogger{}
	}

	o := &RemoteComposeOrchestrator{
		host:     host,
		executor: executor,
		logger:   logger,
	}

	// Apply options
	for _, opt := range opts {
		opt(o)
	}

	// Create default detector if not provided
	if o.detector == nil {
		o.detector = NewComposeDetector(executor, logger)
	}

	// Legacy behavior: if host.Runtime is explicitly set and no compose command
	// was provided via options, create one from the runtime
	if o.composeCmd == nil && host.Runtime != "" {
		o.composeCmd = &ComposeCommand{
			Name:       fmt.Sprintf("%s compose", host.Runtime),
			Binary:     host.Runtime,
			Subcommand: "compose",
		}
		logger.Debug("using host runtime for compose: %s compose", host.Runtime)
	}

	return o
}

// getComposeCommand returns the compose command to use, detecting if necessary.
func (o *RemoteComposeOrchestrator) getComposeCommand(ctx context.Context) (*ComposeCommand, error) {
	// If command was explicitly set, use it
	if o.composeCmd != nil {
		return o.composeCmd, nil
	}

	// Detect once
	o.once.Do(func() {
		o.composeCmd = o.detector.DetectWithFallback(ctx, o.host)
		if o.composeCmd == nil {
			o.detectErr = fmt.Errorf("no compose command detected on host %s", o.host.Name)
		}
	})

	return o.composeCmd, o.detectErr
}

// composeCmdString returns the compose command string for execution.
func (o *RemoteComposeOrchestrator) composeCmdString(ctx context.Context) (string, error) {
	cmd, err := o.getComposeCommand(ctx)
	if err != nil {
		return "", err
	}
	return cmd.String(), nil
}

// Up creates and starts containers on the remote host.
func (o *RemoteComposeOrchestrator) Up(
	ctx context.Context,
	project compose.ComposeProject,
	opts ...compose.UpOption,
) error {
	cmdStr, err := o.composeCmdString(ctx)
	if err != nil {
		return err
	}

	args := o.projectArgs(project)
	args = append(args, "up", "-d")
	if project.Services != nil {
		args = append(args, project.Services...)
	}

	cmd := fmt.Sprintf("%s %s",
		cmdStr, strings.Join(args, " "),
	)
	o.logger.Info("remote compose up on %s: %s",
		o.host.Name, cmd,
	)

	result, err := o.executor.Execute(ctx, o.host, cmd)
	if err != nil {
		return fmt.Errorf(
			"remote compose up on %s: %w", o.host.Name, err,
		)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf(
			"remote compose up on %s: exit %d: %s",
			o.host.Name, result.ExitCode, result.Stderr,
		)
	}
	return nil
}

// Down stops and removes containers on the remote host.
func (o *RemoteComposeOrchestrator) Down(
	ctx context.Context,
	project compose.ComposeProject,
	opts ...compose.DownOption,
) error {
	cmdStr, err := o.composeCmdString(ctx)
	if err != nil {
		return err
	}

	args := o.projectArgs(project)
	args = append(args, "down")

	cmd := fmt.Sprintf("%s %s",
		cmdStr, strings.Join(args, " "),
	)
	o.logger.Info("remote compose down on %s: %s",
		o.host.Name, cmd,
	)

	result, err := o.executor.Execute(ctx, o.host, cmd)
	if err != nil {
		return fmt.Errorf(
			"remote compose down on %s: %w", o.host.Name, err,
		)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf(
			"remote compose down on %s: exit %d: %s",
			o.host.Name, result.ExitCode, result.Stderr,
		)
	}
	return nil
}

// Status returns the status of services on the remote host.
func (o *RemoteComposeOrchestrator) Status(
	ctx context.Context,
	project compose.ComposeProject,
) ([]compose.ServiceStatus, error) {
	cmdStr, err := o.composeCmdString(ctx)
	if err != nil {
		return nil, err
	}

	args := o.projectArgs(project)
	args = append(args, "ps", "--format",
		"'{{.Name}}|{{.State}}|{{.Health}}|{{.Ports}}|{{.ExitCode}}'",
	)

	cmd := fmt.Sprintf("%s %s",
		cmdStr, strings.Join(args, " "),
	)

	result, err := o.executor.Execute(ctx, o.host, cmd)
	if err != nil {
		return nil, fmt.Errorf(
			"remote compose status on %s: %w", o.host.Name, err,
		)
	}

	return parseRemoteComposeStatus(result.Stdout), nil
}

// Logs returns a reader for service log output on the remote host.
func (o *RemoteComposeOrchestrator) Logs(
	ctx context.Context,
	project compose.ComposeProject,
	service string,
) (io.ReadCloser, error) {
	cmdStr, err := o.composeCmdString(ctx)
	if err != nil {
		return nil, err
	}

	args := o.projectArgs(project)
	args = append(args, "logs", "--no-color", service)

	cmd := fmt.Sprintf("%s %s",
		cmdStr, strings.Join(args, " "),
	)

	return o.executor.ExecuteStream(ctx, o.host, cmd)
}

// Host returns the remote host this orchestrator targets.
func (o *RemoteComposeOrchestrator) Host() RemoteHost {
	return o.host
}

// ComposeCommand returns the detected or configured compose command.
func (o *RemoteComposeOrchestrator) ComposeCommand(ctx context.Context) (*ComposeCommand, error) {
	return o.getComposeCommand(ctx)
}

func (o *RemoteComposeOrchestrator) projectArgs(
	project compose.ComposeProject,
) []string {
	var args []string
	if project.File != "" {
		args = append(args, "-f", project.File)
	}
	if project.Name != "" {
		args = append(args, "--project-name", project.Name)
	}
	if project.Profile != "" {
		args = append(args, "--profile", project.Profile)
	}
	return args
}

func parseRemoteComposeStatus(
	output string,
) []compose.ServiceStatus {
	var statuses []compose.ServiceStatus
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "'")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}

		statuses = append(statuses, compose.ServiceStatus{
			Name:   strings.TrimSpace(parts[0]),
			State:  strings.TrimSpace(parts[1]),
			Health: strings.TrimSpace(parts[2]),
		})
	}
	return statuses
}
