package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/runtime"
)

// G1 (CAT-DEVEMU): a refused run option must NEVER reach the remote shell.
func TestRemoteRuntime_Run_RefusedOptionIsNotExecuted(t *testing.T) {
	rt, exe := newTestRemoteRuntime()
	called := false
	exe.executeFunc = func(context.Context, RemoteHost, string) (*CommandResult, error) {
		called = true
		return &CommandResult{}, nil
	}
	_, err := rt.Run(context.Background(), "alpine:3.20", nil,
		runtime.WithRunExtraArgs("--privileged"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, runtime.ErrRunForbiddenOption), "%v", err)
	assert.False(t, called, "remote shell must not be invoked for a refused run")
}
