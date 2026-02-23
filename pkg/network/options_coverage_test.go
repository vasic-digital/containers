package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, 20000, opts.PortRangeStart)
	assert.Equal(t, 30000, opts.PortRangeEnd)
	assert.True(t, opts.AutoReconnect)
	assert.Equal(t, 5*time.Second, opts.ReconnectInterval)
	assert.Equal(t, 10, opts.MaxReconnectAttempts)
}

func TestApplyOptions_Empty(t *testing.T) {
	opts := ApplyOptions(nil)
	defaults := DefaultOptions()
	assert.Equal(t, defaults.PortRangeStart, opts.PortRangeStart)
	assert.Equal(t, defaults.AutoReconnect, opts.AutoReconnect)
}

func TestWithPortRange(t *testing.T) {
	opts := ApplyOptions([]Option{WithPortRange(5000, 6000)})
	assert.Equal(t, 5000, opts.PortRangeStart)
	assert.Equal(t, 6000, opts.PortRangeEnd)
}

func TestWithAutoReconnect(t *testing.T) {
	opts := ApplyOptions([]Option{WithAutoReconnect(false)})
	assert.False(t, opts.AutoReconnect)
}

func TestWithReconnectInterval(t *testing.T) {
	opts := ApplyOptions([]Option{WithReconnectInterval(30 * time.Second)})
	assert.Equal(t, 30*time.Second, opts.ReconnectInterval)
}

func TestWithMaxReconnectAttempts(t *testing.T) {
	opts := ApplyOptions([]Option{WithMaxReconnectAttempts(5)})
	assert.Equal(t, 5, opts.MaxReconnectAttempts)
}

func TestApplyOptions_AllOptions(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithPortRange(10000, 20000),
		WithAutoReconnect(false),
		WithReconnectInterval(10 * time.Second),
		WithMaxReconnectAttempts(3),
	})
	assert.Equal(t, 10000, opts.PortRangeStart)
	assert.Equal(t, 20000, opts.PortRangeEnd)
	assert.False(t, opts.AutoReconnect)
	assert.Equal(t, 10*time.Second, opts.ReconnectInterval)
	assert.Equal(t, 3, opts.MaxReconnectAttempts)
}
