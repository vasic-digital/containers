package volume

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultMountOptions(t *testing.T) {
	opts := DefaultMountOptions()
	assert.NotEmpty(t, opts.SSHFSOptions)
	assert.NotEmpty(t, opts.NFSExportOptions)
	assert.NotEmpty(t, opts.RsyncFlags)
	assert.Greater(t, opts.SyncInterval, time.Duration(0))
}

func TestApplyOptions_Empty(t *testing.T) {
	opts := ApplyOptions(nil)
	defaults := DefaultMountOptions()
	assert.Equal(t, defaults.NFSExportOptions, opts.NFSExportOptions)
	assert.Equal(t, defaults.SyncInterval, opts.SyncInterval)
}

func TestWithSSHFSOptions(t *testing.T) {
	flags := []string{"-o", "uid=1000,gid=1000"}
	opts := ApplyOptions([]Option{WithSSHFSOptions(flags)})
	assert.Equal(t, flags, opts.SSHFSOptions)
}

func TestWithNFSExportOptions(t *testing.T) {
	opts := ApplyOptions([]Option{WithNFSExportOptions("rw,sync,no_root_squash")})
	assert.Equal(t, "rw,sync,no_root_squash", opts.NFSExportOptions)
}

func TestWithRsyncFlags(t *testing.T) {
	flags := []string{"-az", "--compress"}
	opts := ApplyOptions([]Option{WithRsyncFlags(flags)})
	assert.Equal(t, flags, opts.RsyncFlags)
}

func TestWithSyncInterval(t *testing.T) {
	opts := ApplyOptions([]Option{WithSyncInterval(60 * time.Second)})
	assert.Equal(t, 60*time.Second, opts.SyncInterval)
}

func TestApplyOptions_AllOptions(t *testing.T) {
	sshfsFlags := []string{"-o", "allow_other"}
	rsyncFlags := []string{"-avz"}
	opts := ApplyOptions([]Option{
		WithSSHFSOptions(sshfsFlags),
		WithNFSExportOptions("rw,async"),
		WithRsyncFlags(rsyncFlags),
		WithSyncInterval(2 * time.Minute),
	})
	assert.Equal(t, sshfsFlags, opts.SSHFSOptions)
	assert.Equal(t, "rw,async", opts.NFSExportOptions)
	assert.Equal(t, rsyncFlags, opts.RsyncFlags)
	assert.Equal(t, 2*time.Minute, opts.SyncInterval)
}
