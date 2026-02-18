package scheduler

// Options holds configuration for the scheduler.
type Options struct {
	// Strategy is the placement strategy to use.
	Strategy PlacementStrategy
	// LocalHostName is the name representing the local host.
	LocalHostName string
	// CPUWeight is the weight for CPU scoring (0.0-1.0).
	CPUWeight float64
	// MemoryWeight is the weight for memory scoring (0.0-1.0).
	MemoryWeight float64
	// DiskWeight is the weight for disk scoring (0.0-1.0).
	DiskWeight float64
	// NetworkWeight is the weight for network scoring (0.0-1.0).
	NetworkWeight float64
	// OvercommitRatio allows scheduling beyond nominal capacity.
	// A value of 1.5 means 50% overcommit is allowed.
	OvercommitRatio float64
	// ReservePercent is the percentage of host resources to
	// reserve (not schedulable). Default 10%.
	ReservePercent float64
}

// DefaultSchedulerOptions returns sensible defaults.
func DefaultSchedulerOptions() Options {
	return Options{
		Strategy:        StrategyResourceAware,
		LocalHostName:   "local",
		CPUWeight:       0.40,
		MemoryWeight:    0.40,
		DiskWeight:      0.10,
		NetworkWeight:   0.10,
		OvercommitRatio: 1.0,
		ReservePercent:  10.0,
	}
}

// Option configures scheduler behavior.
type Option func(*Options)

// ApplyOptions builds Options from defaults and given options.
func ApplyOptions(opts []Option) Options {
	o := DefaultSchedulerOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithStrategy sets the placement strategy.
func WithStrategy(s PlacementStrategy) Option {
	return func(o *Options) { o.Strategy = s }
}

// WithLocalHostName sets the name representing localhost.
func WithLocalHostName(name string) Option {
	return func(o *Options) { o.LocalHostName = name }
}

// WithCPUWeight sets the CPU scoring weight.
func WithCPUWeight(w float64) Option {
	return func(o *Options) { o.CPUWeight = w }
}

// WithMemoryWeight sets the memory scoring weight.
func WithMemoryWeight(w float64) Option {
	return func(o *Options) { o.MemoryWeight = w }
}

// WithDiskWeight sets the disk scoring weight.
func WithDiskWeight(w float64) Option {
	return func(o *Options) { o.DiskWeight = w }
}

// WithNetworkWeight sets the network scoring weight.
func WithNetworkWeight(w float64) Option {
	return func(o *Options) { o.NetworkWeight = w }
}

// WithOvercommitRatio sets the overcommit ratio.
func WithOvercommitRatio(r float64) Option {
	return func(o *Options) { o.OvercommitRatio = r }
}

// WithReservePercent sets the reserve percentage.
func WithReservePercent(p float64) Option {
	return func(o *Options) { o.ReservePercent = p }
}
