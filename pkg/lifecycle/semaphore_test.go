package lifecycle_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.containers/pkg/lifecycle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrencySemaphore_AcquireRelease(t *testing.T) {
	sem := lifecycle.NewConcurrencySemaphore(2)
	ctx := context.Background()

	require.NoError(t, sem.Acquire(ctx))
	assert.Equal(t, 1, sem.ActiveCount())

	require.NoError(t, sem.Acquire(ctx))
	assert.Equal(t, 2, sem.ActiveCount())

	sem.Release()
	assert.Equal(t, 1, sem.ActiveCount())

	sem.Release()
	assert.Equal(t, 0, sem.ActiveCount())
}

func TestConcurrencySemaphore_Max(t *testing.T) {
	sem := lifecycle.NewConcurrencySemaphore(5)
	assert.Equal(t, 5, sem.Max())
}

func TestConcurrencySemaphore_BlocksWhenFull(t *testing.T) {
	sem := lifecycle.NewConcurrencySemaphore(1)
	ctx := context.Background()

	require.NoError(t, sem.Acquire(ctx))

	// Second acquire should block; use a timeout context.
	timeoutCtx, cancel := context.WithTimeout(
		ctx, 50*time.Millisecond,
	)
	defer cancel()

	err := sem.Acquire(timeoutCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	sem.Release()
}

func TestConcurrencySemaphore_CancelledContext(t *testing.T) {
	sem := lifecycle.NewConcurrencySemaphore(1)
	ctx := context.Background()
	require.NoError(t, sem.Acquire(ctx))

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // cancel immediately

	err := sem.Acquire(cancelCtx)
	assert.Error(t, err)

	sem.Release()
}

func TestConcurrencySemaphore_Concurrent(t *testing.T) {
	const maxConcurrent = 3
	sem := lifecycle.NewConcurrencySemaphore(maxConcurrent)

	var wg sync.WaitGroup
	var maxSeen atomic.Int32
	var current atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if err := sem.Acquire(ctx); err != nil {
				return
			}
			cur := current.Add(1)
			// Track maximum concurrency observed.
			for {
				old := maxSeen.Load()
				if cur <= old {
					break
				}
				if maxSeen.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			current.Add(-1)
			sem.Release()
		}()
	}

	wg.Wait()
	assert.LessOrEqual(t, int(maxSeen.Load()), maxConcurrent)
	assert.Equal(t, 0, sem.ActiveCount())
}

func TestConcurrencySemaphore_ZeroMax(t *testing.T) {
	// Zero should be treated as 1.
	sem := lifecycle.NewConcurrencySemaphore(0)
	assert.Equal(t, 1, sem.Max())
}

func TestConcurrencySemaphore_ReleaseWithoutAcquire(t *testing.T) {
	// Releasing without acquiring should not panic.
	sem := lifecycle.NewConcurrencySemaphore(2)
	sem.Release() // Should not panic; no-op when nothing to release.
	assert.Equal(t, 0, sem.ActiveCount())
}

func TestConcurrencySemaphore_NegativeMax(t *testing.T) {
	// Negative values should be treated as 1.
	sem := lifecycle.NewConcurrencySemaphore(-5)
	assert.Equal(t, 1, sem.Max())
}
