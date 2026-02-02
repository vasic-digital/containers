package compose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"digital.vasic.containers/pkg/logging"
)

// ComposeOrchestrator defines the interface for managing services
// through Docker Compose or compatible orchestration tools.
type ComposeOrchestrator interface {
	// Up creates and starts containers for the given project.
	Up(ctx context.Context, project ComposeProject, opts ...UpOption) error
	// Down stops and removes containers for the given project.
	Down(
		ctx context.Context, project ComposeProject,
		opts ...DownOption,
	) error
	// Status returns the current status of each service in the project.
	Status(
		ctx context.Context, project ComposeProject,
	) ([]ServiceStatus, error)
	// Logs returns a reader streaming log output for the named service.
	Logs(
		ctx context.Context, project ComposeProject, service string,
	) (io.ReadCloser, error)
}

// DefaultOrchestrator implements ComposeOrchestrator by shelling out to
// the detected compose command.
type DefaultOrchestrator struct {
	composeCmd  string
	composeArgs []string
	workDir     string
	logger      logging.Logger
}

// NewDefaultOrchestrator creates a DefaultOrchestrator, auto-detecting
// the available compose command. The workDir is the directory from
// which commands are executed.
func NewDefaultOrchestrator(
	workDir string, logger logging.Logger,
) (*DefaultOrchestrator, error) {
	cmd, args, err := detectComposeCmd()
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = logging.NopLogger{}
	}
	return &DefaultOrchestrator{
		composeCmd:  cmd,
		composeArgs: args,
		workDir:     workDir,
		logger:      logger,
	}, nil
}

// NewOrchestrator creates an orchestrator with an explicit compose
// command and args (useful for testing).
func NewOrchestrator(
	composeCmd string,
	composeArgs []string,
	workDir string,
	logger logging.Logger,
) *DefaultOrchestrator {
	if logger == nil {
		logger = logging.NopLogger{}
	}
	return &DefaultOrchestrator{
		composeCmd:  composeCmd,
		composeArgs: composeArgs,
		workDir:     workDir,
		logger:      logger,
	}
}

// Up creates and starts containers.
func (o *DefaultOrchestrator) Up(
	ctx context.Context,
	project ComposeProject,
	opts ...UpOption,
) error {
	cfg := applyUpOptions(opts)
	args := o.projectArgs(project)
	args = append(args, "up")

	if cfg.Detach {
		args = append(args, "-d")
	}
	if cfg.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if cfg.BuildFirst {
		args = append(args, "--build")
	}
	if cfg.ForceRecreate {
		args = append(args, "--force-recreate")
	}
	if cfg.NoRecreate {
		args = append(args, "--no-recreate")
	}
	if cfg.Timeout > 0 {
		args = append(args, "--timeout",
			strconv.Itoa(cfg.Timeout))
	}
	if cfg.Wait {
		args = append(args, "--wait")
	}

	args = append(args, project.Services...)

	o.logger.Info("compose up: %s %s", o.composeCmd,
		strings.Join(args, " "))
	return o.run(ctx, args)
}

// Down stops and removes containers.
func (o *DefaultOrchestrator) Down(
	ctx context.Context,
	project ComposeProject,
	opts ...DownOption,
) error {
	cfg := applyDownOptions(opts)
	args := o.projectArgs(project)
	args = append(args, "down")

	if cfg.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if cfg.RemoveVolumes {
		args = append(args, "--volumes")
	}
	if cfg.RemoveImages != "" {
		args = append(args, "--rmi", cfg.RemoveImages)
	}
	if cfg.Timeout > 0 {
		args = append(args, "--timeout",
			strconv.Itoa(cfg.Timeout))
	}

	o.logger.Info("compose down: %s %s", o.composeCmd,
		strings.Join(args, " "))
	return o.run(ctx, args)
}

// Status returns the status of all services in the project by parsing
// the output of `docker compose ps`.
func (o *DefaultOrchestrator) Status(
	ctx context.Context,
	project ComposeProject,
) ([]ServiceStatus, error) {
	args := o.projectArgs(project)
	args = append(args, "ps", "--format",
		"{{.Name}}|{{.State}}|{{.Health}}|{{.Ports}}|{{.ExitCode}}")

	out, err := o.output(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("compose ps failed: %w", err)
	}

	return parseStatusOutput(out), nil
}

// Logs returns a reader for the log output of the named service.
func (o *DefaultOrchestrator) Logs(
	ctx context.Context,
	project ComposeProject,
	service string,
) (io.ReadCloser, error) {
	args := o.projectArgs(project)
	args = append(args, "logs", "--no-color", service)

	allArgs := append(o.composeArgs, args...)
	cmd := exec.CommandContext(ctx, o.composeCmd, allArgs...)
	cmd.Dir = o.workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create stdout pipe: %w", err,
		)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"failed to start compose logs: %w", err,
		)
	}

	return &logReader{cmd: cmd, reader: stdout}, nil
}

// logReader wraps a command's stdout pipe and waits for the process to
// exit on Close.
type logReader struct {
	cmd    *exec.Cmd
	reader io.ReadCloser
}

func (r *logReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *logReader) Close() error {
	_ = r.reader.Close()
	return r.cmd.Wait()
}

// projectArgs builds the common compose arguments for a project.
func (o *DefaultOrchestrator) projectArgs(
	project ComposeProject,
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

// run executes the compose command and returns any error.
func (o *DefaultOrchestrator) run(
	ctx context.Context, args []string,
) error {
	allArgs := append(o.composeArgs, args...)
	cmd := exec.CommandContext(ctx, o.composeCmd, allArgs...)
	cmd.Dir = o.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w\nstderr: %s",
			o.composeCmd, strings.Join(allArgs, " "),
			err, stderr.String())
	}
	return nil
}

// output executes the compose command and returns stdout.
func (o *DefaultOrchestrator) output(
	ctx context.Context, args []string,
) (string, error) {
	allArgs := append(o.composeArgs, args...)
	cmd := exec.CommandContext(ctx, o.composeCmd, allArgs...)
	cmd.Dir = o.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s failed: %w\nstderr: %s",
			o.composeCmd, strings.Join(allArgs, " "),
			err, stderr.String())
	}
	return stdout.String(), nil
}

// parseStatusOutput parses the pipe-delimited output from compose ps.
func parseStatusOutput(output string) []ServiceStatus {
	var statuses []ServiceStatus
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}

		exitCode := 0
		if ec, err := strconv.Atoi(
			strings.TrimSpace(parts[4]),
		); err == nil {
			exitCode = ec
		}

		ports := parsePorts(parts[3])

		statuses = append(statuses, ServiceStatus{
			Name:     strings.TrimSpace(parts[0]),
			State:    strings.TrimSpace(parts[1]),
			Health:   strings.TrimSpace(parts[2]),
			Ports:    ports,
			ExitCode: exitCode,
		})
	}
	return statuses
}

// parsePorts splits a comma-separated list of port mappings.
func parsePorts(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// detectComposeCmd tries to find a working compose command, preferring
// Docker Compose v2 plugin, then standalone docker-compose, then
// podman-compose, then podman compose.
func detectComposeCmd() (string, []string, error) {
	candidates := []struct {
		cmd  string
		args []string
	}{
		{"docker", []string{"compose"}},
		{"docker-compose", nil},
		{"podman-compose", nil},
		{"podman", []string{"compose"}},
	}

	for _, c := range candidates {
		checkArgs := append(c.args, "version")
		cmd := exec.Command(c.cmd, checkArgs...)
		if err := cmd.Run(); err == nil {
			return c.cmd, c.args, nil
		}
	}

	return "", nil, fmt.Errorf(
		"no compose command found: tried docker compose, " +
			"docker-compose, podman-compose, podman compose",
	)
}
