package logging

import "log"

// Logger defines a minimal structured logging interface used
// throughout the containers module. Implementations receive a
// message string and optional key-value pairs for structured
// context.
type Logger interface {
	// Debug logs a message at debug level.
	Debug(msg string, args ...any)
	// Info logs a message at info level.
	Info(msg string, args ...any)
	// Warn logs a message at warning level.
	Warn(msg string, args ...any)
	// Error logs a message at error level.
	Error(msg string, args ...any)
}

// StdLogger is a Logger backed by the standard library log package.
type StdLogger struct {
	prefix string
}

// NewStdLogger returns a new StdLogger with the given prefix.
func NewStdLogger(prefix string) *StdLogger {
	return &StdLogger{prefix: prefix}
}

// Debug logs a debug-level message.
func (l *StdLogger) Debug(msg string, args ...any) {
	log.Printf("[DEBUG] %s: "+msg, append([]any{l.prefix}, args...)...)
}

// Info logs an info-level message.
func (l *StdLogger) Info(msg string, args ...any) {
	log.Printf("[INFO]  %s: "+msg, append([]any{l.prefix}, args...)...)
}

// Warn logs a warning-level message.
func (l *StdLogger) Warn(msg string, args ...any) {
	log.Printf("[WARN]  %s: "+msg, append([]any{l.prefix}, args...)...)
}

// Error logs an error-level message.
func (l *StdLogger) Error(msg string, args ...any) {
	log.Printf("[ERROR] %s: "+msg, append([]any{l.prefix}, args...)...)
}

// NopLogger is a Logger that discards all output.
type NopLogger struct{}

// Debug is a no-op.
func (NopLogger) Debug(string, ...any) {}

// Info is a no-op.
func (NopLogger) Info(string, ...any) {}

// Warn is a no-op.
func (NopLogger) Warn(string, ...any) {}

// Error is a no-op.
func (NopLogger) Error(string, ...any) {}
