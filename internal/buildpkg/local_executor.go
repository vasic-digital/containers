package buildpkg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
	"digital.vasic.containers/pkg/remote"
)

// LocalContainerExecutor implements RemoteExecutor by running commands
// inside Podman containers locally (no SSH). Used for local build
// assignments in distributed-build to satisfy §6.K Builds-Inside-Containers.
type LocalContainerExecutor struct {
	runtime      runtime.ContainerRuntime
	projectDir   string
	builderImage string
	logger       logging.Logger
	// userNSKeepID enables --userns=keep-id for permission handling
	userNSKeepID bool
	// buildTimeout bounds each local component build (default 30 min)
	buildTimeout time.Duration
}

// NewLocalContainerExecutor creates a LocalContainerExecutor using the
// provided ContainerRuntime (typically PodmanRuntime from AutoDetect).
func NewLocalContainerExecutor(
	rt runtime.ContainerRuntime,
	projectDir, builderImage string,
) *LocalContainerExecutor {
	return &LocalContainerExecutor{
		runtime:      rt,
		projectDir:   projectDir,
		builderImage: builderImage,
		logger:       logging.NopLogger{},
		userNSKeepID: true,
		buildTimeout: 30 * time.Minute,
	}
}

// SetLogger configures the logger (for testing/observability).
func (e *LocalContainerExecutor) SetLogger(logger logging.Logger) {
	if logger == nil {
		logger = logging.NopLogger{}
	}
	e.logger = logger
}

// SetUserNSKeepID toggles --userns=keep-id (default true).
func (e *LocalContainerExecutor) SetUserNSKeepID(enabled bool) {
	e.userNSKeepID = enabled
}

// SetBuildTimeout sets the per-component build timeout (default 30 min).
func (e *LocalContainerExecutor) SetBuildTimeout(d time.Duration) {
	e.buildTimeout = d
}

// Execute runs a command inside a container using the builder image.
// extraEnv allows passing additional environment variables (e.g., GRADLE_USER_HOME).
func (e *LocalContainerExecutor) Execute(
	ctx context.Context,
	host remote.RemoteHost,
	command string,
	extraEnv map[string]string,
) (*remote.CommandResult, error) {
	start := time.Now()

	// Build run options using the proper RunOption API
	env := map[string]string{
		"JAVA_HOME":   "/opt/java/openjdk",
		"GRADLE_OPTS": "-Dorg.gradle.daemon=false -Dorg.gradle.jvmargs=-Xmx4g",
	}
	for k, v := range extraEnv {
		env[k] = v
	}

	opts := []runtime.RunOption{
		runtime.WithRunRemove(true),
		runtime.WithRunWorkDir("/project"),
		runtime.WithRunVolumes(
			fmt.Sprintf("%s:/project:Z", e.projectDir),
			fmt.Sprintf("%s/releases:/project/releases:Z", e.projectDir),
		),
		runtime.WithRunEnv(env),
		runtime.WithRunUser("root"),
	}

	if e.userNSKeepID {
		opts = append(opts, runtime.WithRunExtraArgs("--userns=keep-id"))
	}

	// Run the command via bash -c
	cmd := []string{"bash", "-c", command}

	e.logger.Debug("podman run local: %s %s", e.builderImage, strings.Join(cmd, " "))

	// Execute via runtime
	result, err := e.runtime.Run(ctx, e.builderImage, cmd, opts...)
	if err != nil {
		// Execution failed (couldn't start container, etc.)
		return &remote.CommandResult{
			Stderr:   err.Error(),
			ExitCode: -1,
			Duration: time.Since(start),
		}, fmt.Errorf("podman run failed: %w", err)
	}

	// Container ran, result contains exit code and output
	return &remote.CommandResult{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
		Duration: time.Since(start),
	}, nil
}

// ExecuteWithOptions runs a command inside a container with custom run options.
func (e *LocalContainerExecutor) ExecuteWithOptions(
	ctx context.Context,
	host remote.RemoteHost,
	command string,
	extraEnv map[string]string,
	extraRunOpts []runtime.RunOption,
) (*remote.CommandResult, error) {
	start := time.Now()

	// Build run options using the proper RunOption API
	env := map[string]string{
		"JAVA_HOME":   "/opt/java/openjdk",
		"GRADLE_OPTS": "-Dorg.gradle.daemon=false -Dorg.gradle.jvmargs=-Xmx4g",
	}
	for k, v := range extraEnv {
		env[k] = v
	}

	opts := []runtime.RunOption{
		runtime.WithRunRemove(true),
		runtime.WithRunWorkDir("/project"),
		runtime.WithRunVolumes(
			fmt.Sprintf("%s:/project:Z", e.projectDir),
			fmt.Sprintf("%s/releases:/project/releases:Z", e.projectDir),
		),
		runtime.WithRunEnv(env),
		runtime.WithRunUser("root"),
	}

	// Add extra run options (volumes, network, etc.)
	opts = append(opts, extraRunOpts...)

	if e.userNSKeepID {
		opts = append(opts, runtime.WithRunExtraArgs("--userns=keep-id"))
	}

	// Run the command via bash -c
	cmd := []string{"bash", "-c", command}

	e.logger.Debug("podman run local: %s %s", e.builderImage, strings.Join(cmd, " "))

	// Execute via runtime
	result, err := e.runtime.Run(ctx, e.builderImage, cmd, opts...)
	if err != nil {
		// Execution failed (couldn't start container, etc.)
		return &remote.CommandResult{
			Stderr:   err.Error(),
			ExitCode: -1,
			Duration: time.Since(start),
		}, fmt.Errorf("podman run failed: %w", err)
	}

	// Container ran, result contains exit code and output
	return &remote.CommandResult{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
		Duration: time.Since(start),
	}, nil
}

// CopyDir is a no-op for local execution since the project is mounted
// directly into the container via volume.
func (e *LocalContainerExecutor) CopyDir(
	ctx context.Context,
	host remote.RemoteHost,
	localDir, remoteDir string,
) error {
	// No-op: project is already available via volume mount
	return nil
}

// IsReachable always returns true for local execution.
func (e *LocalContainerExecutor) IsReachable(
	ctx context.Context,
	host remote.RemoteHost,
) bool {
	return true
}

// ExecuteBuild runs the release-build.sh script for a component inside
// the builder container. This mirrors BuildExecutor.LaunchRemoteBuild
// but executes locally via Podman.
func (e *LocalContainerExecutor) ExecuteBuild(
	ctx context.Context,
	component BuildComponent,
	versionString string,
	skipTests bool,
) (*BuildResult, error) {
	var skipFlag string
	if skipTests {
		skipFlag = " --skip-tests"
	}

	command := fmt.Sprintf(
		"cd /project && git submodule update --init --recursive 2>/dev/null; /project/scripts/release-build.sh --local --component %s --version %s --force%s",
		component.Name, versionString, skipFlag,
	)

	// Use the configured build timeout (default 30 minutes)
	buildCtx, cancel := context.WithTimeout(ctx, e.buildTimeout)
	defer cancel()

	// Create a dummy RemoteHost for the Execute call
	dummyHost := remote.RemoteHost{Name: "local"}

	// Use a component-specific Gradle cache under the pre-existing
	// /gradle-cache directory (created in Dockerfile with 777 perms)
	// to avoid file locking contention between sequential builds
	// (FileHasher timeout).
	gradleUserHome := fmt.Sprintf("/gradle-cache/%s", component.Name)

	// Prepare extra environment variables
	extraEnv := map[string]string{
		"GRADLE_USER_HOME": gradleUserHome,
	}

	// Prepare extra volumes for Go module cache
	extraVolumes := []string{}
	if component.HasGo {
		// Mount host Go module cache and build cache
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			extraVolumes = append(extraVolumes,
				fmt.Sprintf("%s/.cache/go-build:/root/.cache/go-build:Z", homeDir),
				fmt.Sprintf("%s/go/pkg/mod:/root/go/pkg/mod:Z", homeDir),
			)
		}
	}
	// Add component-specific extra volumes
	extraVolumes = append(extraVolumes, component.ExtraVolumes...)

	// Prepare run options with extra volumes and network mode
	runOpts := []runtime.RunOption{}
	if component.NetworkMode != "" {
		runOpts = append(runOpts, runtime.WithRunNetwork(component.NetworkMode))
	}
	if len(extraVolumes) > 0 {
		runOpts = append(runOpts, runtime.WithRunVolumes(extraVolumes...))
	}

	result, err := e.ExecuteWithOptions(buildCtx, dummyHost, command, extraEnv, runOpts)
	if err != nil {
		return &BuildResult{
			Component: component.Name,
			Host:      "local",
			Status:    BuildStatusFailed,
			Duration:  0,
			Error:     err.Error(),
		}, err
	}

	if result.ExitCode != 0 {
		return &BuildResult{
			Component: component.Name,
			Host:      "local",
			Status:    BuildStatusFailed,
			Duration:  result.Duration,
			Error:     fmt.Sprintf("exit code %d: %s", result.ExitCode, truncateString(result.Stderr, 500)),
		}, nil
	}

	return &BuildResult{
		Component: component.Name,
		Host:      "local",
		Status:    BuildStatusSuccess,
		Duration:  result.Duration,
	}, nil
}