package lifecycle

import (
	"context"
	"fmt"
	"sync/atomic"
)

// ConcurrencySemaphore limits the number of concurrent users of a
// service. Callers must Acquire before using the service and
// Release when done.
type ConcurrencySemaphore struct {
	ch     chan struct{}
	max    int
	active atomic.Int32
}

// NewConcurrencySemaphore creates a semaphore allowing up to max
// concurrent acquisitions.
func NewConcurrencySemaphore(max int) *ConcurrencySemaphore {
	if max <= 0 {
		max = 1
	}
	return &ConcurrencySemaphore{
		ch:  make(chan struct{}, max),
		max: max,
	}
}

// Acquire blocks until a slot is available or ctx is cancelled.
func (s *ConcurrencySemaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		s.active.Add(1)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("semaphore: %w", ctx.Err())
	}
}

// Release returns a slot to the semaphore. It must be called
// exactly once for each successful Acquire.
func (s *ConcurrencySemaphore) Release() {
	select {
	case <-s.ch:
		s.active.Add(-1)
	default:
		// No slot to release; this is a programming error but we
		// avoid panicking.
	}
}

// ActiveCount returns the number of currently held slots.
func (s *ConcurrencySemaphore) ActiveCount() int {
	return int(s.active.Load())
}

// Max returns the maximum number of concurrent slots.
func (s *ConcurrencySemaphore) Max() int {
	return s.max
}
