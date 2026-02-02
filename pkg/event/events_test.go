package event

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventType_Constants(t *testing.T) {
	types := []EventType{
		EventContainerStarted,
		EventContainerStopped,
		EventContainerFailed,
		EventHealthChanged,
		EventBootStarted,
		EventBootCompleted,
		EventShutdownStarted,
		EventShutdownCompleted,
	}
	seen := make(map[EventType]bool)
	for _, et := range types {
		assert.NotEmpty(t, string(et))
		assert.False(t, seen[et], "duplicate event type: %s", et)
		seen[et] = true
	}
}

func TestNewEvent(t *testing.T) {
	before := time.Now()
	ev := NewEvent(EventBootStarted, "boot", "system")
	after := time.Now()

	assert.Equal(t, EventBootStarted, ev.Type)
	assert.Equal(t, "boot", ev.Source)
	assert.Equal(t, "system", ev.Name)
	require.NotNil(t, ev.Data)
	assert.Empty(t, ev.Data)
	assert.Nil(t, ev.Error)
	assert.False(t, ev.Timestamp.Before(before))
	assert.False(t, ev.Timestamp.After(after))
}

func TestEvent_WithData(t *testing.T) {
	ev := NewEvent(EventContainerStarted, "runtime", "pg").
		WithData("container_id", "abc123").
		WithData("image", "postgres:15")

	assert.Equal(t, "abc123", ev.Data["container_id"])
	assert.Equal(t, "postgres:15", ev.Data["image"])
}

func TestEvent_WithData_NilMap(t *testing.T) {
	ev := Event{Type: EventContainerStarted}
	ev = ev.WithData("key", "val")
	assert.Equal(t, "val", ev.Data["key"])
}

func TestEvent_WithError(t *testing.T) {
	err := errors.New("connection refused")
	ev := NewEvent(EventContainerFailed, "runtime", "redis").
		WithError(err)
	assert.Equal(t, err, ev.Error)
}

func TestEvent_Immutability(t *testing.T) {
	original := NewEvent(EventBootStarted, "src", "name")
	modified := original.WithData("k", "v")

	assert.Empty(t, original.Data)
	assert.Equal(t, "v", modified.Data["k"])
}
