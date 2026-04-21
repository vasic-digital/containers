package distribution

import (
	"testing"

	"digital.vasic.containers/pkg/logging"
	"github.com/stretchr/testify/assert"
)

func TestApplyOptions_Defaults(t *testing.T) {
	opts := ApplyOptions(nil)
	assert.True(t, opts.EnableTunnels)
	assert.True(t, opts.EnableVolumes)
	assert.NotNil(t, opts.Logger)
}

func TestApplyOptions_WithTunnelsFalse(t *testing.T) {
	opts := ApplyOptions([]Option{WithTunnels(false)})
	assert.False(t, opts.EnableTunnels)
}

func TestApplyOptions_WithVolumesFalse(t *testing.T) {
	opts := ApplyOptions([]Option{WithVolumes(false)})
	assert.False(t, opts.EnableVolumes)
}

func TestWithLocalRuntime_Nil(t *testing.T) {
	opts := ApplyOptions([]Option{WithLocalRuntime(nil)})
	assert.Nil(t, opts.LocalRuntime)
}

func TestWithHostManager_Nil(t *testing.T) {
	opts := ApplyOptions([]Option{WithHostManager(nil)})
	assert.Nil(t, opts.HostManager)
}

func TestWithExecutor_Nil(t *testing.T) {
	opts := ApplyOptions([]Option{WithExecutor(nil)})
	assert.Nil(t, opts.Executor)
}

func TestWithScheduler_Nil(t *testing.T) {
	opts := ApplyOptions([]Option{WithScheduler(nil)})
	assert.Nil(t, opts.Scheduler)
}

func TestWithTunnelManager_Nil(t *testing.T) {
	opts := ApplyOptions([]Option{WithTunnelManager(nil)})
	assert.Nil(t, opts.TunnelManager)
}

func TestWithVolumeManager_Nil(t *testing.T) {
	opts := ApplyOptions([]Option{WithVolumeManager(nil)})
	assert.Nil(t, opts.VolumeManager)
}

func TestWithLogger(t *testing.T) {
	logger := logging.NopLogger{}
	opts := ApplyOptions([]Option{WithLogger(logger)})
	assert.NotNil(t, opts.Logger)
}

func TestWithTunnels(t *testing.T) {
	opts := ApplyOptions([]Option{WithTunnels(false), WithVolumes(false)})
	assert.False(t, opts.EnableTunnels)
	assert.False(t, opts.EnableVolumes)
}
