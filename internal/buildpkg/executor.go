package buildpkg

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.containers/pkg/i18n"
	"digital.vasic.containers/pkg/remote"
)

type RemoteExecutor interface {
	Execute(ctx context.Context, host remote.RemoteHost, command string) (*remote.CommandResult, error)
	CopyDir(ctx context.Context, host remote.RemoteHost, localDir, remoteDir string) error
	IsReachable(ctx context.Context, host remote.RemoteHost) bool
}

type BuildExecutor struct {
	executor     RemoteExecutor
	projectDir   string
	remoteDir    string
	buildTimeout time.Duration
	// translator resolves operator-facing message IDs to localised
	// text per CONST-046. The zero-value NoopTranslator returns the
	// `containers_buildpkg_*` message ID verbatim — positive runtime
	// evidence per CONST-035 / §11.9.
	translator i18n.Translator
}

func NewBuildExecutor(executor RemoteExecutor, projectDir, remoteDir string) *BuildExecutor {
	return &BuildExecutor{
		executor:     executor,
		projectDir:   projectDir,
		remoteDir:    remoteDir,
		buildTimeout: 30 * time.Minute,
		translator:   i18n.NoopTranslator{},
	}
}

func (e *BuildExecutor) WithBuildTimeout(d time.Duration) *BuildExecutor {
	return &BuildExecutor{
		executor:     e.executor,
		projectDir:   e.projectDir,
		remoteDir:    e.remoteDir,
		buildTimeout: d,
		translator:   e.translator,
	}
}

// SetTranslator wires a CONST-046 Translator implementation. Passing
// nil resets to the NoopTranslator default (verbatim message-ID
// fallback). The default constructor already installs NoopTranslator
// so call sites only need this setter to opt into a real bundle
// implementation.
func (e *BuildExecutor) SetTranslator(t i18n.Translator) {
	if t == nil {
		t = i18n.NoopTranslator{}
	}
	e.translator = t
}

func (e *BuildExecutor) SyncSource(ctx context.Context, host remote.RemoteHost) error {
	if !e.executor.IsReachable(ctx, host) {
		return fmt.Errorf("host %s (%s) is not reachable", host.Name, host.Address)
	}

	_, err := e.executor.Execute(ctx, host, fmt.Sprintf("mkdir -p %s", e.remoteDir))
	if err != nil {
		return fmt.Errorf("create remote directory: %w", err)
	}

	err = e.executor.CopyDir(ctx, host, e.projectDir, e.remoteDir)
	if err != nil {
		return fmt.Errorf("copy source to remote: %w", err)
	}

	return nil
}

func (e *BuildExecutor) LaunchRemoteBuild(ctx context.Context, host remote.RemoteHost, component, versionString string, skipTests bool) (*BuildResult, error) {
	if !e.executor.IsReachable(ctx, host) {
		return nil, fmt.Errorf("host %s (%s) is not reachable", host.Name, host.Address)
	}

	var skipFlag string
	if skipTests {
		skipFlag = " --skip-tests"
	}

	command := fmt.Sprintf(
		"cd %s && git submodule update --init --recursive 2>/dev/null; /project/scripts/release-build.sh --local --component %s --force%s",
		e.remoteDir, component, skipFlag,
	)

	buildCtx, cancel := context.WithTimeout(ctx, e.buildTimeout)
	defer cancel()

	result, err := e.executor.Execute(buildCtx, host, command)
	if err != nil {
		return &BuildResult{
			Component: component,
			Host:      host.Name,
			Status:    BuildStatusFailed,
			Duration:  0,
			Error: e.translator.T(ctx, "containers_buildpkg_execution_failed", map[string]any{
				"err": err.Error(),
			}),
		}, err
	}

	if result.ExitCode != 0 {
		return &BuildResult{
			Component: component,
			Host:      host.Name,
			Status:    BuildStatusFailed,
			Duration:  result.Duration,
			Error: e.translator.T(ctx, "containers_buildpkg_exit_code_failed", map[string]any{
				"code":   fmt.Sprintf("%d", result.ExitCode),
				"stderr": truncateString(result.Stderr, 500),
			}),
		}, nil
	}

	return &BuildResult{
		Component: component,
		Host:      host.Name,
		Status:    BuildStatusSuccess,
		Duration:  result.Duration,
	}, nil
}

func (e *BuildExecutor) LaunchLocalBuild(_ context.Context, _ remote.RemoteHost, _, _ string, _ bool) (*BuildResult, error) {
	return nil, fmt.Errorf("local builds are handled by the shell pipeline, not the Go executor")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
