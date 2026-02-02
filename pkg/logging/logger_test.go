package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
