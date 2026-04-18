package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	assert.Equal(t, 10*time.Second, opts.ConnectTimeout)
	assert.Equal(t, 300*time.Second, opts.CommandTimeout)
	assert.Equal(t, 5, opts.MaxConnections)
	assert.Equal(t, 30*time.Second, opts.KeepAlive)
	assert.False(t, opts.StrictHostKeyCheck)
	assert.True(t, opts.ControlMasterEnabled)
	assert.Equal(t, "/tmp/containers-ssh-ctrl", opts.ControlSocketDir)
	assert.Equal(t, 5*time.Minute, opts.ControlPersist)
}

func TestApplyOptions(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithConnectTimeout(20 * time.Second),
		WithCommandTimeout(120 * time.Second),
		WithMaxConnections(10),
		WithKeepAlive(60 * time.Second),
		WithStrictHostKeyCheck(true),
		WithControlMaster(false),
		WithControlSocketDir("/var/run/ssh-ctrl"),
		WithControlPersist(10 * time.Minute),
	})

	assert.Equal(t, 20*time.Second, opts.ConnectTimeout)
	assert.Equal(t, 120*time.Second, opts.CommandTimeout)
	assert.Equal(t, 10, opts.MaxConnections)
	assert.Equal(t, 60*time.Second, opts.KeepAlive)
	assert.True(t, opts.StrictHostKeyCheck)
	assert.False(t, opts.ControlMasterEnabled)
	assert.Equal(t, "/var/run/ssh-ctrl", opts.ControlSocketDir)
	assert.Equal(t, 10*time.Minute, opts.ControlPersist)
}

func TestApplyOptions_Empty(t *testing.T) {
	opts := ApplyOptions(nil)
	defaults := DefaultOptions()

	assert.Equal(t, defaults.ConnectTimeout, opts.ConnectTimeout)
	assert.Equal(t, defaults.CommandTimeout, opts.CommandTimeout)
	assert.Equal(t, defaults.MaxConnections, opts.MaxConnections)
}

func TestWithConnectTimeout(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithConnectTimeout(5 * time.Second),
	})
	assert.Equal(t, 5*time.Second, opts.ConnectTimeout)
}

func TestWithCommandTimeout(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithCommandTimeout(30 * time.Second),
	})
	assert.Equal(t, 30*time.Second, opts.CommandTimeout)
}

func TestWithMaxConnections(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithMaxConnections(20),
	})
	assert.Equal(t, 20, opts.MaxConnections)
}

func TestWithKeepAlive(t *testing.T) {
	opts := ApplyOptions([]Option{
		WithKeepAlive(45 * time.Second),
	})
	assert.Equal(t, 45*time.Second, opts.KeepAlive)
}
