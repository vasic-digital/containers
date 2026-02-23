package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSchedulerOptions(t *testing.T) {
	opts := DefaultSchedulerOptions()
	assert.Equal(t, StrategyResourceAware, opts.Strategy)
	assert.Equal(t, "local", opts.LocalHostName)
	assert.Equal(t, 0.40, opts.CPUWeight)
	assert.Equal(t, 0.40, opts.MemoryWeight)
	assert.Equal(t, 0.10, opts.DiskWeight)
	assert.Equal(t, 0.10, opts.NetworkWeight)
	assert.Equal(t, 1.0, opts.OvercommitRatio)
	assert.Equal(t, 10.0, opts.ReservePercent)
}

func TestApplyOptions_Empty(t *testing.T) {
	opts := ApplyOptions(nil)
	defaults := DefaultSchedulerOptions()
	assert.Equal(t, defaults.Strategy, opts.Strategy)
	assert.Equal(t, defaults.LocalHostName, opts.LocalHostName)
}

func TestWithStrategy(t *testing.T) {
	opts := ApplyOptions([]Option{WithStrategy(StrategyRoundRobin)})
	assert.Equal(t, StrategyRoundRobin, opts.Strategy)
}

func TestWithLocalHostName(t *testing.T) {
	opts := ApplyOptions([]Option{WithLocalHostName("my-host")})
	assert.Equal(t, "my-host", opts.LocalHostName)
}

func TestWithCPUWeight(t *testing.T) {
	opts := ApplyOptions([]Option{WithCPUWeight(0.5)})
	assert.Equal(t, 0.5, opts.CPUWeight)
}

func TestWithMemoryWeight(t *testing.T) {
	opts := ApplyOptions([]Option{WithMemoryWeight(0.3)})
	assert.Equal(t, 0.3, opts.MemoryWeight)
}

func TestWithDiskWeight(t *testing.T) {
	opts := ApplyOptions([]Option{WithDiskWeight(0.2)})
	assert.Equal(t, 0.2, opts.DiskWeight)
}

func TestWithNetworkWeight(t *testing.T) {
	opts := ApplyOptions([]Option{WithNetworkWeight(0.15)})
	assert.Equal(t, 0.15, opts.NetworkWeight)
}

func TestWithOvercommitRatio(t *testing.T) {
	opts := ApplyOptions([]Option{WithOvercommitRatio(1.5)})
	assert.Equal(t, 1.5, opts.OvercommitRatio)
}

func TestWithReservePercent(t *testing.T) {
	opts := ApplyOptions([]Option{WithReservePercent(20.0)})
	assert.Equal(t, 20.0, opts.ReservePercent)
}

func TestApplyOptions_AllOptions(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithStrategy(StrategyBinPack),
		WithLocalHostName("host-1"),
		WithCPUWeight(0.6),
		WithMemoryWeight(0.2),
		WithDiskWeight(0.1),
		WithNetworkWeight(0.1),
		WithOvercommitRatio(1.2),
		WithReservePercent(5.0),
	})
	assert.Equal(t, StrategyBinPack, opts.Strategy)
	assert.Equal(t, "host-1", opts.LocalHostName)
	assert.Equal(t, 0.6, opts.CPUWeight)
	assert.Equal(t, 0.2, opts.MemoryWeight)
	assert.Equal(t, 0.1, opts.DiskWeight)
	assert.Equal(t, 0.1, opts.NetworkWeight)
	assert.Equal(t, 1.2, opts.OvercommitRatio)
	assert.Equal(t, 5.0, opts.ReservePercent)
}
