package distribution

import (
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/network"
	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/runtime"
	"digital.vasic.containers/pkg/scheduler"
	"digital.vasic.containers/pkg/volume"
)

// Options configures the distributor.
type Options struct {
	// LocalRuntime is the container runtime for local execution.
	LocalRuntime runtime.ContainerRuntime
	// HostManager manages remote hosts.
	HostManager remote.HostManager
	// Executor executes commands on remote hosts.
	Executor remote.RemoteExecutor
	// Scheduler selects hosts for containers.
	Scheduler scheduler.Scheduler
	// TunnelManager creates SSH tunnels.
	TunnelManager network.TunnelManager
	// VolumeManager handles remote volume mounts.
	VolumeManager volume.VolumeManager
	// Logger is the logging interface.
	Logger logging.Logger
	// EnableTunnels controls whether SSH tunnels are created.
	EnableTunnels bool
	// EnableVolumes controls whether volumes are mounted.
	EnableVolumes bool
}

// Option configures distribution behavior.
type Option func(*Options)

// ApplyOptions builds Options from given options.
func ApplyOptions(opts []Option) Options {
	o := Options{
		EnableTunnels: true,
		EnableVolumes: true,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.Logger == nil {
		o.Logger = logging.NopLogger{}
	}
	return o
}

// WithLocalRuntime sets the local container runtime.
func WithLocalRuntime(r runtime.ContainerRuntime) Option {
	return func(o *Options) { o.LocalRuntime = r }
}

// WithHostManager sets the host manager.
func WithHostManager(hm remote.HostManager) Option {
	return func(o *Options) { o.HostManager = hm }
}

// WithExecutor sets the remote executor.
func WithExecutor(e remote.RemoteExecutor) Option {
	return func(o *Options) { o.Executor = e }
}

// WithScheduler sets the container scheduler.
func WithScheduler(s scheduler.Scheduler) Option {
	return func(o *Options) { o.Scheduler = s }
}

// WithTunnelManager sets the SSH tunnel manager.
func WithTunnelManager(tm network.TunnelManager) Option {
	return func(o *Options) { o.TunnelManager = tm }
}

// WithVolumeManager sets the remote volume manager.
func WithVolumeManager(vm volume.VolumeManager) Option {
	return func(o *Options) { o.VolumeManager = vm }
}

// WithLogger sets the logger.
func WithLogger(l logging.Logger) Option {
	return func(o *Options) { o.Logger = l }
}

// WithTunnels enables or disables SSH tunnel creation.
func WithTunnels(enabled bool) Option {
	return func(o *Options) { o.EnableTunnels = enabled }
}

// WithVolumes enables or disables volume mounting.
func WithVolumes(enabled bool) Option {
	return func(o *Options) { o.EnableVolumes = enabled }
}
