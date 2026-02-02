package event

import "time"

// EventType identifies the kind of lifecycle or operational event.
type EventType string

const (
	// EventContainerStarted is emitted when a container starts.
	EventContainerStarted EventType = "container.started"
	// EventContainerStopped is emitted when a container stops.
	EventContainerStopped EventType = "container.stopped"
	// EventContainerFailed is emitted when a container fails.
	EventContainerFailed EventType = "container.failed"
	// EventHealthChanged is emitted when a health status changes.
	EventHealthChanged EventType = "health.changed"
	// EventBootStarted is emitted when the boot sequence begins.
	EventBootStarted EventType = "boot.started"
	// EventBootCompleted is emitted when boot finishes.
	EventBootCompleted EventType = "boot.completed"
	// EventShutdownStarted is emitted when shutdown begins.
	EventShutdownStarted EventType = "shutdown.started"
	// EventShutdownCompleted is emitted when shutdown finishes.
	EventShutdownCompleted EventType = "shutdown.completed"
)

// Event represents a single system event carrying contextual
// data about a container or lifecycle state change.
type Event struct {
	// Type identifies the event kind.
	Type EventType
	// Timestamp is when the event occurred.
	Timestamp time.Time
	// Source identifies the component that produced the event.
	Source string
	// Name is a human-readable label for the event subject
	// (e.g., container name).
	Name string
	// Data holds arbitrary key-value pairs associated with the
	// event.
	Data map[string]any
	// Error holds an optional error related to the event.
	Error error
}

// NewEvent creates an Event with the given type and source,
// timestamped to the current time.
func NewEvent(t EventType, source, name string) Event {
	return Event{
		Type:      t,
		Timestamp: time.Now(),
		Source:    source,
		Name:     name,
		Data:     make(map[string]any),
	}
}

// WithData returns a copy of the event with the given key-value
// pair added to Data. The original event's Data map is not
// modified.
func (e Event) WithData(key string, value any) Event {
	newData := make(map[string]any, len(e.Data)+1)
	for k, v := range e.Data {
		newData[k] = v
	}
	newData[key] = value
	e.Data = newData
	return e
}

// WithError returns a copy of the event with the error set.
func (e Event) WithError(err error) Event {
	e.Error = err
	return e
}
