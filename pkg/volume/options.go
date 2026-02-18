package volume

import "time"

// Option configures volume management behavior.
type Option func(*MountOptions)

// ApplyOptions builds MountOptions from defaults and given options.
func ApplyOptions(opts []Option) MountOptions {
	o := DefaultMountOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithSSHFSOptions sets extra sshfs flags.
func WithSSHFSOptions(flags []string) Option {
	return func(o *MountOptions) { o.SSHFSOptions = flags }
}

// WithNFSExportOptions sets NFS export options.
func WithNFSExportOptions(opts string) Option {
	return func(o *MountOptions) { o.NFSExportOptions = opts }
}

// WithRsyncFlags sets extra rsync flags.
func WithRsyncFlags(flags []string) Option {
	return func(o *MountOptions) { o.RsyncFlags = flags }
}

// WithSyncInterval sets the rsync synchronization period.
func WithSyncInterval(d time.Duration) Option {
	return func(o *MountOptions) { o.SyncInterval = d }
}
