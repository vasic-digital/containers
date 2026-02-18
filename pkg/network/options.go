package network

import "time"

// Options configures network tunnel behavior.
type Options struct {
	// PortRangeStart is the start of the local port range.
	PortRangeStart int
	// PortRangeEnd is the end of the local port range.
	PortRangeEnd int
	// AutoReconnect enables automatic tunnel reconnection.
	AutoReconnect bool
	// ReconnectInterval is the delay between reconnect attempts.
	ReconnectInterval time.Duration
	// MaxReconnectAttempts limits reconnect retries (0=unlimited).
	MaxReconnectAttempts int
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		PortRangeStart:       20000,
		PortRangeEnd:         30000,
		AutoReconnect:        true,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 10,
	}
}

// Option configures network behavior.
type Option func(*Options)

// ApplyOptions builds Options from defaults and given options.
func ApplyOptions(opts []Option) Options {
	o := DefaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithPortRange sets the local port allocation range.
func WithPortRange(start, end int) Option {
	return func(o *Options) {
		o.PortRangeStart = start
		o.PortRangeEnd = end
	}
}

// WithAutoReconnect enables or disables automatic reconnection.
func WithAutoReconnect(enabled bool) Option {
	return func(o *Options) { o.AutoReconnect = enabled }
}

// WithReconnectInterval sets the delay between reconnect
// attempts.
func WithReconnectInterval(d time.Duration) Option {
	return func(o *Options) { o.ReconnectInterval = d }
}

// WithMaxReconnectAttempts sets the maximum reconnect retries.
func WithMaxReconnectAttempts(n int) Option {
	return func(o *Options) { o.MaxReconnectAttempts = n }
}
