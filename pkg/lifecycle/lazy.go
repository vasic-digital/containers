package lifecycle

import "sync"

// LazyBooter ensures a service is started exactly once, deferring
// the start until the first call to EnsureStarted.
type LazyBooter struct {
	once    sync.Once
	startFn func() error
	err     error
}

// NewLazyBooter creates a LazyBooter that will invoke startFn on
// the first call to EnsureStarted.
func NewLazyBooter(startFn func() error) *LazyBooter {
	return &LazyBooter{startFn: startFn}
}

// EnsureStarted calls the start function at most once. Subsequent
// calls return the result of the first invocation.
func (lb *LazyBooter) EnsureStarted() error {
	lb.once.Do(func() {
		if lb.startFn != nil {
			lb.err = lb.startFn()
		}
	})
	return lb.err
}

// Started reports whether the start function has been executed.
func (lb *LazyBooter) Started() bool {
	// A simple heuristic: if once.Do already ran, calling it again
	// with a no-op will be a no-op. We track this via the err
	// field and a secondary flag.
	ch := make(chan struct{})
	ran := false
	lb.once.Do(func() {
		ran = true
		close(ch)
	})
	if ran {
		// The once hadn't fired yet; we just triggered it with a
		// no-op. This means the original startFn was nil or was
		// never called. Return false.
		return false
	}
	return true
}
