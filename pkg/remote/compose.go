package remote

import (
	"context"
	"fmt"
	"io"
	"strings"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/logging"
)

// RemoteComposeOrchestrator implements compose.ComposeOrchestrator
// by executing compose commands on a remote host via SSH.
type RemoteComposeOrchestrator struct {
	host       RemoteHost
	executor   RemoteExecutor
	composeCmd string
	logger     logging.Logger
}

// NewRemoteComposeOrchestrator creates a compose orchestrator that
// operates on a remote host.
func NewRemoteComposeOrchestrator(
	host RemoteHost,
	executor RemoteExecutor,
	logger logging.Logger,
) *RemoteComposeOrchestrator {
	if logger == nil {
		logger = logging.NopLogger{}
	}
	composeCmd := "docker compose"
	if host.Runtime == "podman" {
		composeCmd = "podman compose"
	}
	return &RemoteComposeOrchestrator{
		host:       host,
		executor:   executor,
		composeCmd: composeCmd,
		logger:     logger,
	}
}

// Up creates and starts containers on the remote host.
func (o *RemoteComposeOrchestrator) Up(
	ctx context.Context,
	project compose.ComposeProject,
	opts ...compose.UpOption,
) error {
	args := o.projectArgs(project)
	args = append(args, "up", "-d")
	if project.Services != nil {
		args = append(args, project.Services...)
	}

	cmd := fmt.Sprintf("%s %s",
		o.composeCmd, strings.Join(args, " "),
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
	args := o.projectArgs(project)
	args = append(args, "down")

	cmd := fmt.Sprintf("%s %s",
		o.composeCmd, strings.Join(args, " "),
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
	args := o.projectArgs(project)
	args = append(args, "ps", "--format",
		"'{{.Name}}|{{.State}}|{{.Health}}|{{.Ports}}|{{.ExitCode}}'",
	)

	cmd := fmt.Sprintf("%s %s",
		o.composeCmd, strings.Join(args, " "),
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
	args := o.projectArgs(project)
	args = append(args, "logs", "--no-color", service)

	cmd := fmt.Sprintf("%s %s",
		o.composeCmd, strings.Join(args, " "),
	)

	return o.executor.ExecuteStream(ctx, o.host, cmd)
}

// Host returns the remote host this orchestrator targets.
func (o *RemoteComposeOrchestrator) Host() RemoteHost {
	return o.host
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
