package logging

import (
	"context"
	"log/slog"
)

// SlogAdapter wraps a *slog.Logger to satisfy the Logger
// interface.
type SlogAdapter struct {
	logger *slog.Logger
}

// Ensure SlogAdapter satisfies Logger at compile time.
var _ Logger = (*SlogAdapter)(nil)

// NewSlogAdapter creates a SlogAdapter from the given slog
// logger. If logger is nil, slog.Default() is used.
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogAdapter{logger: logger}
}

// Debug logs a message at debug level.
func (a *SlogAdapter) Debug(msg string, args ...any) {
	a.logger.Log(context.Background(), slog.LevelDebug, msg, args...)
}

// Info logs a message at info level.
func (a *SlogAdapter) Info(msg string, args ...any) {
	a.logger.Log(context.Background(), slog.LevelInfo, msg, args...)
}

// Warn logs a message at warning level.
func (a *SlogAdapter) Warn(msg string, args ...any) {
	a.logger.Log(context.Background(), slog.LevelWarn, msg, args...)
}

// Error logs a message at error level.
func (a *SlogAdapter) Error(msg string, args ...any) {
	a.logger.Log(context.Background(), slog.LevelError, msg, args...)
}
