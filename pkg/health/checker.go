package health

import (
	"context"
	"sync"
	"time"
)

// CheckFunc performs a health check against a single target.
type CheckFunc func(ctx context.Context, target HealthTarget) *HealthResult

// HealthChecker defines the interface for performing health checks.
type HealthChecker interface {
	// Check performs a health check on a single target.
	Check(ctx context.Context, target HealthTarget) *HealthResult
	// CheckAll performs health checks on all targets concurrently and
	// returns the collected results.
	CheckAll(ctx context.Context, targets []HealthTarget) []*HealthResult
}

// DefaultChecker is a HealthChecker that dispatches to registered
// CheckFunc implementations based on the target's HealthType.
type DefaultChecker struct {
	mu       sync.RWMutex
	checkers map[HealthType]CheckFunc
}

// NewDefaultChecker creates a DefaultChecker pre-loaded with the
// built-in TCP, HTTP, and gRPC check functions.
func NewDefaultChecker() *DefaultChecker {
	c := &DefaultChecker{
		checkers: make(map[HealthType]CheckFunc),
	}
	c.checkers[HealthTCP] = CheckTCP
	c.checkers[HealthHTTP] = CheckHTTP
	c.checkers[HealthGRPC] = CheckGRPC
	return c
}

// Register adds or replaces the CheckFunc for the given HealthType.
func (c *DefaultChecker) Register(t HealthType, f CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkers[t] = f
}

// Check performs a health check for the given target using the registered
// CheckFunc for its type. If no function is registered, the result is
// marked unhealthy with a descriptive error.
func (c *DefaultChecker) Check(
	ctx context.Context,
	target HealthTarget,
) *HealthResult {
	c.mu.RLock()
	fn, ok := c.checkers[target.Type]
	c.mu.RUnlock()

	if !ok {
		return &HealthResult{
			Target:    target.Name,
			Healthy:   false,
			Error:     "no checker registered for type: " + string(target.Type),
			Timestamp: time.Now(),
		}
	}

	if target.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, target.Timeout)
		defer cancel()
	}

	return fn(ctx, target)
}

// CheckAll runs health checks for all targets concurrently and returns
// the results in the same order as the input slice.
func (c *DefaultChecker) CheckAll(
	ctx context.Context,
	targets []HealthTarget,
) []*HealthResult {
	results := make([]*HealthResult, len(targets))
	var wg sync.WaitGroup
	wg.Add(len(targets))

	for i, t := range targets {
		go func(idx int, target HealthTarget) {
			defer wg.Done()
			results[idx] = c.Check(ctx, target)
		}(i, t)
	}

	wg.Wait()
	return results
}
