package health

import (
	"context"
	"time"
)

// RetryPolicy configures how many times and how long to wait between
// successive health check attempts.
type RetryPolicy struct {
	// MaxRetries is the maximum number of additional attempts after the
	// first failure. A value of 0 means no retries.
	MaxRetries int
	// Delay is the base duration to wait between attempts.
	Delay time.Duration
	// BackoffFactor multiplies the delay after each failed attempt. A
	// value of 1.0 keeps the delay constant.
	BackoffFactor float64
}

// DefaultRetryPolicy returns a sensible default retry configuration.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:    3,
		Delay:         1 * time.Second,
		BackoffFactor: 2.0,
	}
}

// CheckWithRetry performs a health check using the given checker and
// target, retrying according to the policy until the check succeeds or
// all attempts are exhausted. The last HealthResult is always returned.
func CheckWithRetry(
	ctx context.Context,
	checker HealthChecker,
	target HealthTarget,
	policy RetryPolicy,
) *HealthResult {
	var result *HealthResult
	delay := policy.Delay

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		result = checker.Check(ctx, target)
		if result.Healthy {
			return result
		}

		// Don't sleep after the last attempt.
		if attempt < policy.MaxRetries {
			select {
			case <-ctx.Done():
				result.Error = "context cancelled during retry: " + result.Error
				return result
			case <-time.After(delay):
			}

			if policy.BackoffFactor > 0 {
				delay = time.Duration(
					float64(delay) * policy.BackoffFactor,
				)
			}
		}
	}

	return result
}
