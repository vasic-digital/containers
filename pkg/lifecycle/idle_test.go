package lifecycle_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.containers/pkg/lifecycle"

	"github.com/stretchr/testify/assert"
)

func TestIdleShutdown_Fires(t *testing.T) {
	var fired atomic.Int32
	is := lifecycle.NewIdleShutdown(
		100*time.Millisecond,
		func() { fired.Add(1) },
	)

	is.Start()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), fired.Load())
}

func TestIdleShutdown_TouchResets(t *testing.T) {
	var fired atomic.Int32
	is := lifecycle.NewIdleShutdown(
		150*time.Millisecond,
		func() { fired.Add(1) },
	)

	is.Start()
	time.Sleep(100 * time.Millisecond)
	is.Touch() // reset: should not fire yet
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), fired.Load())

	// Wait for the full timeout after touch.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), fired.Load())
}

func TestIdleShutdown_StopPrevents(t *testing.T) {
	var fired atomic.Int32
	is := lifecycle.NewIdleShutdown(
		100*time.Millisecond,
		func() { fired.Add(1) },
	)

	is.Start()
	is.Stop()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), fired.Load())
}

func TestIdleShutdown_TouchAfterStop(t *testing.T) {
	is := lifecycle.NewIdleShutdown(
		100*time.Millisecond,
		func() {},
	)
	is.Stop()
	// Should not panic.
	is.Touch()
}

func TestIdleShutdown_LastTouch(t *testing.T) {
	is := lifecycle.NewIdleShutdown(
		time.Hour, func() {},
	)

	is.Start()
	defer is.Stop()

	t1 := is.LastTouch()
	time.Sleep(10 * time.Millisecond)
	is.Touch()
	t2 := is.LastTouch()

	assert.True(t, t2.After(t1))
}

func TestIdleShutdown_MultipleStarts(t *testing.T) {
	var fired atomic.Int32
	is := lifecycle.NewIdleShutdown(
		100*time.Millisecond,
		func() { fired.Add(1) },
	)

	is.Start()
	is.Start() // should reset
	time.Sleep(200 * time.Millisecond)
	// Should only fire once.
	assert.Equal(t, int32(1), fired.Load())
}

func TestIdleShutdown_NilCallback(t *testing.T) {
	// Test that nil onIdle callback does not panic.
	is := lifecycle.NewIdleShutdown(
		50*time.Millisecond,
		nil,
	)

	is.Start()
	time.Sleep(100 * time.Millisecond)
	// Should not panic; the fire() method checks for nil.
}

func TestIdleShutdown_TouchBeforeStart(t *testing.T) {
	var fired atomic.Int32
	is := lifecycle.NewIdleShutdown(
		100*time.Millisecond,
		func() { fired.Add(1) },
	)

	// Touch before Start should be a no-op (timer is nil).
	is.Touch()

	is.Start()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), fired.Load())
}

func TestIdleShutdown_StopMultipleTimes(t *testing.T) {
	is := lifecycle.NewIdleShutdown(
		100*time.Millisecond,
		func() {},
	)

	is.Start()
	is.Stop()
	// Calling Stop again should not panic.
	is.Stop()
}

func TestIdleShutdown_LastTouchBeforeStart(t *testing.T) {
	is := lifecycle.NewIdleShutdown(
		time.Hour, func() {},
	)

	// LastTouch before Start returns zero time.
	t1 := is.LastTouch()
	assert.True(t, t1.IsZero())
}

func TestIdleShutdown_FireAfterStopRace(t *testing.T) {
	// This test covers the race condition between timer firing and Stop().
	// With a nanosecond timeout, the timer may fire before Stop() acquires
	// the lock. In this case, the callback legitimately runs because it
	// acquired the lock first and checked stopped=false.
	//
	// The key invariant is: callback fires AT MOST once per Start(), and
	// after Stop() returns, no additional callbacks occur.

	for i := 0; i < 100; i++ {
		var fired atomic.Int32
		is := lifecycle.NewIdleShutdown(
			time.Nanosecond, // Very short to increase race chance.
			func() { fired.Add(1) },
		)

		is.Start()
		// Immediately stop to race with the timer.
		is.Stop()

		// Wait for any pending timer to complete.
		time.Sleep(time.Millisecond)

		// The callback should fire at most once (0 or 1).
		// It may be 1 if the timer fired before Stop() acquired the lock.
		count := fired.Load()
		assert.True(t, count <= 1, "callback should fire at most once, got %d", count)

		// Verify no additional callbacks occur after Stop() + sleep.
		time.Sleep(time.Millisecond)
		assert.Equal(t, count, fired.Load(), "no additional callbacks after Stop()")
	}
}

func TestIdleShutdown_FireIdempotent(t *testing.T) {
	// Test that once idle shutdown fires, it sets stopped to true
	// and subsequent timer events (if any) are ignored.
	var fired atomic.Int32
	is := lifecycle.NewIdleShutdown(
		50*time.Millisecond,
		func() { fired.Add(1) },
	)

	is.Start()
	// Wait for first fire.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), fired.Load())

	// Start again without explicitly calling Stop (this should reset).
	is.Start()
	time.Sleep(100 * time.Millisecond)
	// Should fire again since we called Start.
	assert.Equal(t, int32(2), fired.Load())
}

func TestIdleShutdown_ConcurrentStartStop(t *testing.T) {
	// Stress test: concurrent Start and Stop calls to cover race paths.
	var wg sync.WaitGroup
	var fired atomic.Int32

	for i := 0; i < 50; i++ {
		is := lifecycle.NewIdleShutdown(
			time.Microsecond,
			func() { fired.Add(1) },
		)

		wg.Add(2)
		go func() {
			defer wg.Done()
			is.Start()
		}()
		go func() {
			defer wg.Done()
			time.Sleep(time.Microsecond)
			is.Stop()
		}()
	}

	wg.Wait()
	// Wait for any pending timers.
	time.Sleep(10 * time.Millisecond)
	// We don't assert specific count - just ensure no panic/race.
}
