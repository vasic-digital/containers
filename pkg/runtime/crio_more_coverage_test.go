//go:build !integration

package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCRIO_Remove_Force exercises the `if o.Force` branch in CRI-O
// Remove by passing WithForceRemove, which adds the "-f" flag.
func TestCRIO_Remove_Force(t *testing.T) {
	var capturedArgs []string
	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, name string, args ...string,
		) ([]byte, error) {
			capturedArgs = args
			return []byte(""), nil
		},
	}
	c := NewCRIORuntimeWithExecutor(exec)

	err := c.Remove(context.Background(), "container-id",
		WithForceRemove(true))
	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "-f",
		"crictl rm should include -f when Force is true")
}

// TestCRIO_Remove_Error exercises the error branch in CRI-O Remove
// when the executor returns an error.
func TestCRIO_Remove_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, name string, args ...string,
		) ([]byte, error) {
			return nil, fmt.Errorf("crictl rm: not found")
		},
	}
	c := NewCRIORuntimeWithExecutor(exec)

	err := c.Remove(context.Background(), "container-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crictl rm container-id")
}
