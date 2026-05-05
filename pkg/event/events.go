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

	// Remote host events.

	// EventRemoteHostOnline is emitted when a remote host comes
	// online.
	EventRemoteHostOnline EventType = "remote.host.online"
	// EventRemoteHostOffline is emitted when a remote host goes
	// offline.
	EventRemoteHostOffline EventType = "remote.host.offline"
	// EventRemoteHostDegraded is emitted when a remote host is
	// resource-constrained.
	EventRemoteHostDegraded EventType = "remote.host.degraded"

	// Distribution events.

	// EventDistributionScheduled is emitted when containers are
	// scheduled for placement.
	EventDistributionScheduled EventType = "distribution.scheduled"
	// EventDistributionDeployed is emitted when a container is
	// deployed to a host.
	EventDistributionDeployed EventType = "distribution.deployed"
	// EventDistributionMigrated is emitted when a container is
	// migrated between hosts.
	EventDistributionMigrated EventType = "distribution.migrated"
	// EventDistributionStarted is emitted when a distribution
	// workflow begins.
	EventDistributionStarted EventType = "distribution.started"
	// EventDistributionCompleted is emitted when a distribution
	// workflow finishes.
	EventDistributionCompleted EventType = "distribution.completed"

	// Tunnel events.

	// EventTunnelCreated is emitted when an SSH tunnel is created.
	EventTunnelCreated EventType = "tunnel.created"
	// EventTunnelClosed is emitted when an SSH tunnel is closed.
	EventTunnelClosed EventType = "tunnel.closed"
	// EventTunnelFailed is emitted when an SSH tunnel fails.
	EventTunnelFailed EventType = "tunnel.failed"

	// Volume events.

	// EventVolumeMounted is emitted when a remote volume is
	// mounted.
	EventVolumeMounted EventType = "volume.mounted"
	// EventVolumeUnmounted is emitted when a remote volume is
	// unmounted.
	EventVolumeUnmounted EventType = "volume.unmounted"
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
		Name:      name,
		Data:      make(map[string]any),
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
