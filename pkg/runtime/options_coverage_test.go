//go:build !integration

package runtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplyStartOptions_Defaults(t *testing.T) {
	opts := applyStartOptions(nil)
	assert.True(t, opts.Detach)
	assert.False(t, opts.Remove)
	assert.NotNil(t, opts.Env)
}

func TestWithDetach(t *testing.T) {
	opts := applyStartOptions([]StartOption{WithDetach(false)})
	assert.False(t, opts.Detach)
}

func TestWithRemoveOnExit(t *testing.T) {
	opts := applyStartOptions([]StartOption{WithRemoveOnExit(true)})
	assert.True(t, opts.Remove)
}

func TestWithEnv(t *testing.T) {
	env := map[string]string{"FOO": "bar", "BAZ": "qux"}
	opts := applyStartOptions([]StartOption{WithEnv(env)})
	assert.Equal(t, "bar", opts.Env["FOO"])
	assert.Equal(t, "qux", opts.Env["BAZ"])
}

func TestApplyStopOptions_Defaults(t *testing.T) {
	opts := applyStopOptions(nil)
	assert.Equal(t, 10*time.Second, opts.Timeout)
}

func TestWithStopTimeout(t *testing.T) {
	opts := applyStopOptions([]StopOption{WithStopTimeout(30 * time.Second)})
	assert.Equal(t, 30*time.Second, opts.Timeout)
}

func TestApplyRemoveOptions_Defaults(t *testing.T) {
	opts := applyRemoveOptions(nil)
	assert.False(t, opts.Force)
	assert.False(t, opts.Volumes)
}

func TestWithForceRemove(t *testing.T) {
	opts := applyRemoveOptions([]RemoveOption{WithForceRemove(true)})
	assert.True(t, opts.Force)
}

func TestWithRemoveVolumes(t *testing.T) {
	opts := applyRemoveOptions([]RemoveOption{WithRemoveVolumes(true)})
	assert.True(t, opts.Volumes)
}

func TestApplyLogOptions_Defaults(t *testing.T) {
	opts := applyLogOptions(nil)
	assert.False(t, opts.Follow)
	assert.Equal(t, "all", opts.Tail)
	assert.Empty(t, opts.Since)
	assert.Empty(t, opts.Until)
}

func TestWithFollow(t *testing.T) {
	opts := applyLogOptions([]LogOption{WithFollow(true)})
	assert.True(t, opts.Follow)
}

func TestWithSince(t *testing.T) {
	opts := applyLogOptions([]LogOption{WithSince("1h")})
	assert.Equal(t, "1h", opts.Since)
}

func TestWithUntil(t *testing.T) {
	opts := applyLogOptions([]LogOption{WithUntil("2024-01-01")})
	assert.Equal(t, "2024-01-01", opts.Until)
}

func TestWithTail(t *testing.T) {
	opts := applyLogOptions([]LogOption{WithTail("100")})
	assert.Equal(t, "100", opts.Tail)
}
