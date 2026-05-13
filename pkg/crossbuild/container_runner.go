package crossbuild

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// containerRunner is the seam for invoking podman/docker. Production
// uses realContainerRunner (auto-detects via pkg/runtime); tests
// inject a fake.
type containerRunner interface {
	ImageExists(ctx context.Context, imageRef string) bool
	Run(ctx context.Context, spec containerRunSpec) (exitCode int, err error)
}

// containerRunSpec is the minimum surface needed by the crossbuild
// orchestrator. Intentionally LESS than pkg/runtime's full surface
// to avoid coupling crossbuild's tests to runtime's internal state.
type containerRunSpec struct {
	Image       string
	MountSource string
	MountTarget string
	WorkDir     string
	Command     string
	Environment map[string]string
	Stdout      *bytes.Buffer
	Stderr      *bytes.Buffer
}

type realContainerRunner struct{}

func (realContainerRunner) ImageExists(ctx context.Context, imageRef string) bool {
	binary := pickContainerBinary()
	if binary == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, binary, "image", "exists", imageRef)
	return cmd.Run() == nil
}

func (realContainerRunner) Run(ctx context.Context, spec containerRunSpec) (int, error) {
	binary := pickContainerBinary()
	if binary == "" {
		return -1, fmt.Errorf("neither podman nor docker is installed on this host")
	}
	args := []string{
		"run", "--rm", "--read-only", "--tmpfs", "/tmp:rw",
		"-v", spec.MountSource + ":" + spec.MountTarget + ":Z",
		"-w", spec.WorkDir,
	}
	for k, v := range spec.Environment {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, spec.Image, "sh", "-c", spec.Command)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	err := cmd.Run()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return exitCode, err
}

// pickContainerBinary mirrors pkg/runtime auto-detection but kept
// inline so pkg/crossbuild does not introduce a hard import on
// pkg/runtime (which would create a build-time cycle with the
// distribution package). The rule is the same: prefer rootless
// podman, fall back to docker.
func pickContainerBinary() string {
	for _, bin := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(bin); err == nil {
			return bin
		}
	}
	return ""
}
