package crossbuild

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// HostDirectBackend executes the BuildCommand on the host directly, no
// container or VM. It is the fast path for host-native targets
// (Linux→Linux, Darwin→Darwin, etc.) where virtualisation would only
// add overhead.
//
// Anti-bluff: this backend is the OPERATIONAL one — it actually runs
// the build. Tests do NOT mock os/exec; they assert orchestration
// via the processRunner seam below.
type HostDirectBackend struct {
	runner processRunner
}

// NewHostDirectBackend returns the production backend.
func NewHostDirectBackend() *HostDirectBackend {
	return &HostDirectBackend{runner: realRunner{}}
}

// newHostDirectBackendWithRunner is the test seam.
func newHostDirectBackendWithRunner(r processRunner) *HostDirectBackend {
	return &HostDirectBackend{runner: r}
}

func (h *HostDirectBackend) Name() string { return "host-direct" }

func (h *HostDirectBackend) Capabilities() Capabilities {
	return Capabilities{
		// Host-direct supports the current host's GOOS/GOARCH only.
		// The Selector relies on this list being accurate.
		SupportsTargets: []Target{
			{OS: runtime.GOOS, Arch: runtime.GOARCH},
		},
		RequiresHostOS:      nil, // works on every host
		IsolatesEnvironment: false,
		ArtifactNotes:       "produces native artifact for host OS/arch (no virtualisation)",
	}
}

func (h *HostDirectBackend) Build(ctx context.Context, req BuildRequest) BuildResult {
	start := time.Now()
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := validateRequest(req); err != nil {
		return BuildResult{Target: req.Target, BackendName: h.Name(), Error: err, Duration: time.Since(start)}
	}

	var stdout, stderr bytes.Buffer
	exitCode, err := h.runner.Run(ctx, req.SourceDir, req.BuildCommand, req.Environment, &stdout, &stderr)

	result := BuildResult{
		Target:      req.Target,
		BackendName: h.Name(),
		StdoutTail:  tailString(stdout.String(), 4096),
		StderrTail:  tailString(stderr.String(), 4096),
		Duration:    time.Since(start),
	}
	if err != nil {
		result.Error = fmt.Errorf("build command failed (exit=%d): %w", exitCode, err)
		return result
	}
	if exitCode != 0 {
		result.Error = fmt.Errorf("build command exited %d", exitCode)
		return result
	}

	produced := filepath.Join(req.SourceDir, req.OutputSubpath)
	stat, err := os.Stat(produced)
	if err != nil {
		result.Error = fmt.Errorf(
			"build command succeeded but artifact missing at %s: %w "+
				"(anti-bluff: a 'BUILD SUCCESSFUL' without a real artifact is a bluff)",
			produced, err)
		return result
	}
	if stat.Size() == 0 {
		result.Error = fmt.Errorf(
			"build produced a zero-byte artifact at %s "+
				"(anti-bluff: empty artifact == bluff)", produced)
		return result
	}

	// Copy from SourceDir/OutputSubpath to HostOutputDir/<basename>.
	dst := filepath.Join(req.HostOutputDir, filepath.Base(produced))
	if err := copyFile(produced, dst); err != nil {
		result.Error = fmt.Errorf("copying artifact to HostOutputDir: %w", err)
		return result
	}
	result.ArtifactPath = dst
	result.ArtifactSize = stat.Size()
	return result
}

// processRunner is the seam for tests. Production uses realRunner.
type processRunner interface {
	Run(ctx context.Context, dir, command string, env map[string]string,
		stdout, stderr *bytes.Buffer) (exitCode int, err error)
}

type realRunner struct{}

func (realRunner) Run(ctx context.Context, dir, command string, env map[string]string,
	stdout, stderr *bytes.Buffer) (int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	if env != nil {
		envSlice := os.Environ()
		for k, v := range env {
			envSlice = append(envSlice, k+"="+v)
		}
		cmd.Env = envSlice
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return exitCode, err
}

func validateRequest(req BuildRequest) error {
	if req.SourceDir == "" {
		return fmt.Errorf("crossbuild: BuildRequest.SourceDir is required")
	}
	if !filepath.IsAbs(req.SourceDir) {
		return fmt.Errorf("crossbuild: BuildRequest.SourceDir must be absolute, got %q", req.SourceDir)
	}
	if _, err := os.Stat(req.SourceDir); err != nil {
		return fmt.Errorf("crossbuild: BuildRequest.SourceDir not accessible: %w", err)
	}
	if req.BuildCommand == "" {
		return fmt.Errorf("crossbuild: BuildRequest.BuildCommand is required")
	}
	if req.OutputSubpath == "" {
		return fmt.Errorf("crossbuild: BuildRequest.OutputSubpath is required")
	}
	if req.HostOutputDir == "" {
		return fmt.Errorf("crossbuild: BuildRequest.HostOutputDir is required")
	}
	if err := os.MkdirAll(req.HostOutputDir, 0o755); err != nil {
		return fmt.Errorf("crossbuild: HostOutputDir not creatable: %w", err)
	}
	return nil
}

func tailString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return err
	}
	return nil
}
