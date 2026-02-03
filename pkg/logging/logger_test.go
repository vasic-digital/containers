package logging

import (
	"bytes"
	"log"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// StdLogger Tests
// =============================================================================

func TestNewStdLogger(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "empty prefix",
			prefix: "",
		},
		{
			name:   "simple prefix",
			prefix: "myapp",
		},
		{
			name:   "prefix with spaces",
			prefix: "my app",
		},
		{
			name:   "prefix with special chars",
			prefix: "app:v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewStdLogger(tt.prefix)
			require.NotNil(t, logger)
			assert.Equal(t, tt.prefix, logger.prefix)
		})
	}
}

func TestStdLogger_ImplementsLogger(t *testing.T) {
	var l Logger = NewStdLogger("test")
	require.NotNil(t, l)
}

func TestStdLogger_Debug(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		msg            string
		args           []any
		expectedParts  []string
	}{
		{
			name:          "simple message",
			prefix:        "test",
			msg:           "debug message",
			args:          nil,
			expectedParts: []string{"[DEBUG]", "test:", "debug message"},
		},
		{
			name:          "message with format args",
			prefix:        "app",
			msg:           "value=%d",
			args:          []any{42},
			expectedParts: []string{"[DEBUG]", "app:", "value=42"},
		},
		{
			name:          "message with string arg",
			prefix:        "svc",
			msg:           "key=%s",
			args:          []any{"val"},
			expectedParts: []string{"[DEBUG]", "svc:", "key=val"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetFlags(0) // Remove timestamp for predictable output
			defer log.SetOutput(nil)
			defer log.SetFlags(log.LstdFlags)

			logger := NewStdLogger(tt.prefix)
			logger.Debug(tt.msg, tt.args...)

			out := buf.String()
			for _, part := range tt.expectedParts {
				assert.Contains(t, out, part)
			}
		})
	}
}

func TestStdLogger_Info(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		msg            string
		args           []any
		expectedParts  []string
	}{
		{
			name:          "simple message",
			prefix:        "test",
			msg:           "info message",
			args:          nil,
			expectedParts: []string{"[INFO]", "test:", "info message"},
		},
		{
			name:          "message with format args",
			prefix:        "app",
			msg:           "count=%d",
			args:          []any{100},
			expectedParts: []string{"[INFO]", "app:", "count=100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetFlags(0)
			defer log.SetOutput(nil)
			defer log.SetFlags(log.LstdFlags)

			logger := NewStdLogger(tt.prefix)
			logger.Info(tt.msg, tt.args...)

			out := buf.String()
			for _, part := range tt.expectedParts {
				assert.Contains(t, out, part)
			}
		})
	}
}

func TestStdLogger_Warn(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		msg            string
		args           []any
		expectedParts  []string
	}{
		{
			name:          "simple message",
			prefix:        "test",
			msg:           "warn message",
			args:          nil,
			expectedParts: []string{"[WARN]", "test:", "warn message"},
		},
		{
			name:          "message with format args",
			prefix:        "svc",
			msg:           "threshold=%f",
			args:          []any{0.95},
			expectedParts: []string{"[WARN]", "svc:", "threshold="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetFlags(0)
			defer log.SetOutput(nil)
			defer log.SetFlags(log.LstdFlags)

			logger := NewStdLogger(tt.prefix)
			logger.Warn(tt.msg, tt.args...)

			out := buf.String()
			for _, part := range tt.expectedParts {
				assert.Contains(t, out, part)
			}
		})
	}
}

func TestStdLogger_Error(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		msg            string
		args           []any
		expectedParts  []string
	}{
		{
			name:          "simple message",
			prefix:        "test",
			msg:           "error message",
			args:          nil,
			expectedParts: []string{"[ERROR]", "test:", "error message"},
		},
		{
			name:          "message with error arg",
			prefix:        "db",
			msg:           "connection failed: %s",
			args:          []any{"timeout"},
			expectedParts: []string{"[ERROR]", "db:", "connection failed: timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetFlags(0)
			defer log.SetOutput(nil)
			defer log.SetFlags(log.LstdFlags)

			logger := NewStdLogger(tt.prefix)
			logger.Error(tt.msg, tt.args...)

			out := buf.String()
			for _, part := range tt.expectedParts {
				assert.Contains(t, out, part)
			}
		})
	}
}

func TestStdLogger_AllLevels(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(nil)
	defer log.SetFlags(log.LstdFlags)

	logger := NewStdLogger("multi")

	logger.Debug("d msg")
	logger.Info("i msg")
	logger.Warn("w msg")
	logger.Error("e msg")

	out := buf.String()
	assert.Contains(t, out, "[DEBUG]")
	assert.Contains(t, out, "[INFO]")
	assert.Contains(t, out, "[WARN]")
	assert.Contains(t, out, "[ERROR]")
	assert.Contains(t, out, "multi:")
}

// =============================================================================
// NopLogger Tests (from logger.go - distinct from NoopLogger in noop.go)
// =============================================================================

func TestNopLogger_ImplementsLogger(t *testing.T) {
	var l Logger = NopLogger{}
	require.NotNil(t, l)
}

func TestNopLogger_Debug(t *testing.T) {
	logger := NopLogger{}
	// Should not panic with any arguments
	assert.NotPanics(t, func() {
		logger.Debug("message")
	})
	assert.NotPanics(t, func() {
		logger.Debug("message", "key", "value")
	})
	assert.NotPanics(t, func() {
		logger.Debug("message", "k1", 1, "k2", 2, "k3", 3)
	})
}

func TestNopLogger_Info(t *testing.T) {
	logger := NopLogger{}
	assert.NotPanics(t, func() {
		logger.Info("message")
	})
	assert.NotPanics(t, func() {
		logger.Info("message", "key", "value")
	})
}

func TestNopLogger_Warn(t *testing.T) {
	logger := NopLogger{}
	assert.NotPanics(t, func() {
		logger.Warn("message")
	})
	assert.NotPanics(t, func() {
		logger.Warn("message", "key", "value")
	})
}

func TestNopLogger_Error(t *testing.T) {
	logger := NopLogger{}
	assert.NotPanics(t, func() {
		logger.Error("message")
	})
	assert.NotPanics(t, func() {
		logger.Error("message", "key", "value")
	})
}

func TestNopLogger_DoesNotWrite(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	logger := NopLogger{}
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	// NopLogger should not write anything to the standard logger
	assert.Empty(t, buf.String())
}

// =============================================================================
// NoopLogger Tests (from noop.go)
// =============================================================================

func TestNoopLogger_ImplementsLogger(t *testing.T) {
	var l Logger = &NoopLogger{}
	require.NotNil(t, l)

	// Must not panic.
	l.Debug("msg", "key", "val")
	l.Info("msg", "key", "val")
	l.Warn("msg", "key", "val")
	l.Error("msg", "key", "val")
}

func TestSlogAdapter_ImplementsLogger(t *testing.T) {
	var l Logger = NewSlogAdapter(nil)
	require.NotNil(t, l)
}

func TestSlogAdapter_NilFallsBackToDefault(t *testing.T) {
	adapter := NewSlogAdapter(nil)
	require.NotNil(t, adapter.logger)
}

func TestSlogAdapter_Debug(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	adapter := NewSlogAdapter(slog.New(handler))

	adapter.Debug("hello debug", "count", 42)

	out := buf.String()
	assert.Contains(t, out, "hello debug")
	assert.Contains(t, out, "count=42")
	assert.Contains(t, out, "DEBUG")
}

func TestSlogAdapter_Info(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	adapter := NewSlogAdapter(slog.New(handler))

	adapter.Info("hello info", "service", "pg")

	out := buf.String()
	assert.Contains(t, out, "hello info")
	assert.Contains(t, out, "service=pg")
	assert.Contains(t, out, "INFO")
}

func TestSlogAdapter_Warn(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	adapter := NewSlogAdapter(slog.New(handler))

	adapter.Warn("hello warn")

	out := buf.String()
	assert.Contains(t, out, "hello warn")
	assert.Contains(t, out, "WARN")
}

func TestSlogAdapter_Error(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	adapter := NewSlogAdapter(slog.New(handler))

	adapter.Error("hello error", "err", "timeout")

	out := buf.String()
	assert.Contains(t, out, "hello error")
	assert.Contains(t, out, "err=timeout")
	assert.Contains(t, out, "ERROR")
}

func TestSlogAdapter_AllLevels(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	adapter := NewSlogAdapter(slog.New(handler))

	adapter.Debug("d")
	adapter.Info("i")
	adapter.Warn("w")
	adapter.Error("e")

	out := buf.String()
	assert.Contains(t, out, "DEBUG")
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "WARN")
	assert.Contains(t, out, "ERROR")
}

// =============================================================================
// Additional NoopLogger Tests (comprehensive coverage for noop.go)
// =============================================================================

func TestNoopLogger_Debug(t *testing.T) {
	logger := NoopLogger{}
	tests := []struct {
		name string
		msg  string
		args []any
	}{
		{name: "empty message", msg: "", args: nil},
		{name: "simple message", msg: "debug", args: nil},
		{name: "with args", msg: "debug", args: []any{"key", "value"}},
		{name: "with many args", msg: "debug", args: []any{"k1", 1, "k2", "v2", "k3", true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				logger.Debug(tt.msg, tt.args...)
			})
		})
	}
}

func TestNoopLogger_Info(t *testing.T) {
	logger := NoopLogger{}
	tests := []struct {
		name string
		msg  string
		args []any
	}{
		{name: "empty message", msg: "", args: nil},
		{name: "simple message", msg: "info", args: nil},
		{name: "with args", msg: "info", args: []any{"key", "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				logger.Info(tt.msg, tt.args...)
			})
		})
	}
}

func TestNoopLogger_Warn(t *testing.T) {
	logger := NoopLogger{}
	tests := []struct {
		name string
		msg  string
		args []any
	}{
		{name: "empty message", msg: "", args: nil},
		{name: "simple message", msg: "warn", args: nil},
		{name: "with args", msg: "warn", args: []any{"key", "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				logger.Warn(tt.msg, tt.args...)
			})
		})
	}
}

func TestNoopLogger_Error(t *testing.T) {
	logger := NoopLogger{}
	tests := []struct {
		name string
		msg  string
		args []any
	}{
		{name: "empty message", msg: "", args: nil},
		{name: "simple message", msg: "error", args: nil},
		{name: "with args", msg: "error", args: []any{"err", "timeout"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				logger.Error(tt.msg, tt.args...)
			})
		})
	}
}

func TestNoopLogger_DoesNotWrite(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	logger := NoopLogger{}
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// NoopLogger should not write anything
	assert.Empty(t, buf.String())
}

func TestNoopLogger_CompileTimeCheck(t *testing.T) {
	// This test verifies the compile-time interface check in noop.go
	var _ Logger = (*NoopLogger)(nil)
}
