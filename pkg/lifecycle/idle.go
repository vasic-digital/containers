package lifecycle

import (
	"sync"
	"time"
)

// IdleShutdown monitors a service for inactivity and triggers a
// callback when the idle timeout elapses without any Touch calls.
type IdleShutdown struct {
	mu       sync.Mutex
	timeout  time.Duration
	onIdle   func()
	timer    *time.Timer
	stopped  bool
	lastTouch time.Time
}

// NewIdleShutdown creates an IdleShutdown that fires onIdle after
// timeout elapses without a Touch.
func NewIdleShutdown(timeout time.Duration, onIdle func()) *IdleShutdown {
	return &IdleShutdown{
		timeout: timeout,
		onIdle:  onIdle,
	}
}

// Start begins the idle countdown. If Start has already been called
// it resets the timer.
func (is *IdleShutdown) Start() {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.stopped = false
	is.lastTouch = time.Now()

	if is.timer != nil {
		is.timer.Stop()
	}
	is.timer = time.AfterFunc(is.timeout, is.fire)
}

// Touch resets the idle countdown to the full timeout duration.
func (is *IdleShutdown) Touch() {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.stopped || is.timer == nil {
		return
	}
	is.lastTouch = time.Now()
	is.timer.Reset(is.timeout)
}

// Stop cancels the idle countdown. The onIdle callback will not be
// invoked after Stop returns.
func (is *IdleShutdown) Stop() {
	is.mu.Lock()
	defer is.mu.Unlock()

	is.stopped = true
	if is.timer != nil {
		is.timer.Stop()
		is.timer = nil
	}
}

// LastTouch returns the time of the most recent Touch (or Start).
func (is *IdleShutdown) LastTouch() time.Time {
	is.mu.Lock()
	defer is.mu.Unlock()
	return is.lastTouch
}

// fire is called by the timer when the idle period elapses.
func (is *IdleShutdown) fire() {
	is.mu.Lock()
	if is.stopped {
		is.mu.Unlock()
		return
	}
	is.stopped = true
	is.mu.Unlock()

	if is.onIdle != nil {
		is.onIdle()
	}
}
