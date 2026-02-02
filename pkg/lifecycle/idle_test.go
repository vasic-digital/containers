package lifecycle_test

import (
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
