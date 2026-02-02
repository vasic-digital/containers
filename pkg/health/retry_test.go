package health

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckWithRetry_SucceedsFirstAttempt(t *testing.T) {
	c := NewDefaultChecker()
	var calls int32
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		atomic.AddInt32(&calls, 1)
		return &HealthResult{
			Target:    target.Name,
			Healthy:   true,
			Timestamp: time.Now(),
		}
	})

	policy := RetryPolicy{MaxRetries: 3, Delay: 10 * time.Millisecond}
	target := HealthTarget{Name: "retry-ok", Type: HealthCustom}

	ctx := context.Background()
	result := CheckWithRetry(ctx, c, target, policy)

	assert.True(t, result.Healthy)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestCheckWithRetry_SucceedsAfterRetries(t *testing.T) {
	c := NewDefaultChecker()
	var calls int32
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return &HealthResult{
				Target:    target.Name,
				Healthy:   false,
				Error:     "not ready yet",
				Timestamp: time.Now(),
			}
		}
		return &HealthResult{
			Target:    target.Name,
			Healthy:   true,
			Timestamp: time.Now(),
		}
	})

	policy := RetryPolicy{
		MaxRetries:    5,
		Delay:         10 * time.Millisecond,
		BackoffFactor: 1.0,
	}
	target := HealthTarget{Name: "retry-eventual", Type: HealthCustom}

	ctx := context.Background()
	result := CheckWithRetry(ctx, c, target, policy)

	assert.True(t, result.Healthy)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestCheckWithRetry_ExhaustsRetries(t *testing.T) {
	c := NewDefaultChecker()
	var calls int32
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		atomic.AddInt32(&calls, 1)
		return &HealthResult{
			Target:    target.Name,
			Healthy:   false,
			Error:     "still failing",
			Timestamp: time.Now(),
		}
	})

	policy := RetryPolicy{
		MaxRetries:    2,
		Delay:         10 * time.Millisecond,
		BackoffFactor: 1.0,
	}
	target := HealthTarget{Name: "retry-exhaust", Type: HealthCustom}

	ctx := context.Background()
	result := CheckWithRetry(ctx, c, target, policy)

	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "still failing")
	// 1 initial + 2 retries = 3 total.
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestCheckWithRetry_RespectsContextCancellation(t *testing.T) {
	c := NewDefaultChecker()
	var calls int32
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		atomic.AddInt32(&calls, 1)
		return &HealthResult{
			Target:    target.Name,
			Healthy:   false,
			Error:     "failing",
			Timestamp: time.Now(),
		}
	})

	policy := RetryPolicy{
		MaxRetries:    100,
		Delay:         50 * time.Millisecond,
		BackoffFactor: 1.0,
	}
	target := HealthTarget{Name: "retry-cancel", Type: HealthCustom}

	ctx, cancel := context.WithTimeout(
		context.Background(), 120*time.Millisecond,
	)
	defer cancel()

	result := CheckWithRetry(ctx, c, target, policy)

	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "context cancelled during retry")
	// Should have made only a few attempts before context expired.
	assert.Less(t, atomic.LoadInt32(&calls), int32(10))
}

func TestCheckWithRetry_BackoffFactor(t *testing.T) {
	c := NewDefaultChecker()
	var timestamps []time.Time
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		timestamps = append(timestamps, time.Now())
		return &HealthResult{
			Target:    target.Name,
			Healthy:   false,
			Error:     "failing",
			Timestamp: time.Now(),
		}
	})

	policy := RetryPolicy{
		MaxRetries:    3,
		Delay:         50 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	target := HealthTarget{Name: "retry-backoff", Type: HealthCustom}

	ctx := context.Background()
	result := CheckWithRetry(ctx, c, target, policy)

	assert.False(t, result.Healthy)
	require.Len(t, timestamps, 4) // 1 initial + 3 retries

	// Verify increasing delays. Allow tolerance for scheduling jitter.
	d1 := timestamps[1].Sub(timestamps[0])
	d2 := timestamps[2].Sub(timestamps[1])
	d3 := timestamps[3].Sub(timestamps[2])

	assert.Greater(t, d2.Milliseconds(), d1.Milliseconds()/2,
		"second delay should be longer than half of first")
	assert.Greater(t, d3.Milliseconds(), d2.Milliseconds()/2,
		"third delay should be longer than half of second")
}

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	assert.Equal(t, 3, p.MaxRetries)
	assert.Equal(t, 1*time.Second, p.Delay)
	assert.Equal(t, 2.0, p.BackoffFactor)
}
