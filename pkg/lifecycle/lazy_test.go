package lifecycle_test

import (
	"errors"
	"sync"
	"testing"

	"digital.vasic.containers/pkg/lifecycle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLazyBooter_EnsureStarted_Success(t *testing.T) {
	calls := 0
	lb := lifecycle.NewLazyBooter(func() error {
		calls++
		return nil
	})

	require.NoError(t, lb.EnsureStarted())
	require.NoError(t, lb.EnsureStarted()) // idempotent
	assert.Equal(t, 1, calls)
}

func TestLazyBooter_EnsureStarted_Error(t *testing.T) {
	expectedErr := errors.New("boot failed")
	lb := lifecycle.NewLazyBooter(func() error {
		return expectedErr
	})

	err := lb.EnsureStarted()
	assert.ErrorIs(t, err, expectedErr)

	// Subsequent calls return the same error.
	err = lb.EnsureStarted()
	assert.ErrorIs(t, err, expectedErr)
}

func TestLazyBooter_EnsureStarted_NilFunc(t *testing.T) {
	lb := lifecycle.NewLazyBooter(nil)
	assert.NoError(t, lb.EnsureStarted())
}

func TestLazyBooter_ConcurrentCalls(t *testing.T) {
	calls := 0
	var mu sync.Mutex
	lb := lifecycle.NewLazyBooter(func() error {
		mu.Lock()
		calls++
		mu.Unlock()
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = lb.EnsureStarted()
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, calls)
}

func TestLazyBooter_Started_AfterEnsureStarted(t *testing.T) {
	lb := lifecycle.NewLazyBooter(func() error {
		return nil
	})

	// Before calling EnsureStarted, Started should return false.
	// Note: Due to implementation, calling Started() before
	// EnsureStarted triggers the once.Do with a no-op.
	// So we need to test after EnsureStarted is called.

	require.NoError(t, lb.EnsureStarted())
	assert.True(t, lb.Started())
}

func TestLazyBooter_Started_NotStartedYet(t *testing.T) {
	lb := lifecycle.NewLazyBooter(func() error {
		return nil
	})

	// Calling Started on a fresh LazyBooter will trigger the
	// once.Do with a no-op, so it returns false.
	assert.False(t, lb.Started())
}

func TestLazyBooter_Started_NilFunc(t *testing.T) {
	lb := lifecycle.NewLazyBooter(nil)

	// Started() on a fresh booter with nil startFn.
	started := lb.Started()
	// The once.Do ran during Started() call, so it returns false.
	assert.False(t, started)
}
