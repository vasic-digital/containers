package logging

// NoopLogger is a Logger that silently discards all output. It
// is useful as a default when no logging is configured.
type NoopLogger struct{}

// Ensure NoopLogger satisfies Logger at compile time.
var _ Logger = (*NoopLogger)(nil)

// Debug is a no-op.
func (NoopLogger) Debug(string, ...any) {}

// Info is a no-op.
func (NoopLogger) Info(string, ...any) {}

// Warn is a no-op.
func (NoopLogger) Warn(string, ...any) {}

// Error is a no-op.
func (NoopLogger) Error(string, ...any) {}
