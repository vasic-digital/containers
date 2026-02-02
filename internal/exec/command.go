package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Run executes the named program with the given arguments, capturing
// both standard output and standard error. The command inherits the
// current working directory.
func Run(
	ctx context.Context,
	name string,
	args ...string,
) (stdout, stderr string, err error) {
	return RunInDir(ctx, "", name, args...)
}

// RunInDir executes the named program with the given arguments in the
// specified directory. If dir is empty the current working directory
// is used.
func RunInDir(
	ctx context.Context,
	dir, name string,
	args ...string,
) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if runErr := cmd.Run(); runErr != nil {
		return outBuf.String(), errBuf.String(), fmt.Errorf(
			"exec %s: %w (stderr: %s)",
			name, runErr, errBuf.String(),
		)
	}
	return outBuf.String(), errBuf.String(), nil
}
