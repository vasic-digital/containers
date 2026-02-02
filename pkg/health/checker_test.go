package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultChecker_RegistersBuiltins(t *testing.T) {
	c := NewDefaultChecker()
	require.NotNil(t, c)
	assert.Contains(t, c.checkers, HealthTCP)
	assert.Contains(t, c.checkers, HealthHTTP)
	assert.Contains(t, c.checkers, HealthGRPC)
}

func TestDefaultChecker_Register_OverridesExisting(t *testing.T) {
	c := NewDefaultChecker()
	called := false
	custom := func(_ context.Context, _ HealthTarget) *HealthResult {
		called = true
		return &HealthResult{Healthy: true, Timestamp: time.Now()}
	}
	c.Register(HealthTCP, custom)

	ctx := context.Background()
	c.Check(ctx, HealthTarget{Name: "test", Type: HealthTCP})
	assert.True(t, called, "custom checker should have been called")
}

func TestDefaultChecker_Check_UnknownType(t *testing.T) {
	c := NewDefaultChecker()
	ctx := context.Background()
	result := c.Check(ctx, HealthTarget{
		Name: "unknown",
		Type: HealthType("kafka"),
	})
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "no checker registered")
}

func TestDefaultChecker_Check_RespectsTimeout(t *testing.T) {
	c := NewDefaultChecker()
	called := false
	c.Register(HealthCustom, func(
		ctx context.Context, _ HealthTarget,
	) *HealthResult {
		called = true
		dl, ok := ctx.Deadline()
		assert.True(t, ok, "context should have a deadline")
		assert.WithinDuration(t, time.Now().Add(50*time.Millisecond),
			dl, 100*time.Millisecond)
		return &HealthResult{Healthy: true, Timestamp: time.Now()}
	})

	ctx := context.Background()
	c.Check(ctx, HealthTarget{
		Name:    "timeout-test",
		Type:    HealthCustom,
		Timeout: 50 * time.Millisecond,
	})
	assert.True(t, called)
}

func TestDefaultChecker_CheckAll_Concurrent(t *testing.T) {
	c := NewDefaultChecker()
	results := make([]string, 0, 3)
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		return &HealthResult{
			Target:    target.Name,
			Healthy:   true,
			Timestamp: time.Now(),
		}
	})

	targets := []HealthTarget{
		{Name: "a", Type: HealthCustom},
		{Name: "b", Type: HealthCustom},
		{Name: "c", Type: HealthCustom},
	}

	ctx := context.Background()
	all := c.CheckAll(ctx, targets)
	require.Len(t, all, 3)

	for _, r := range all {
		results = append(results, r.Target)
		assert.True(t, r.Healthy)
	}
	assert.ElementsMatch(t, []string{"a", "b", "c"}, results)
}

func TestDefaultChecker_CheckAll_PreservesOrder(t *testing.T) {
	c := NewDefaultChecker()
	c.Register(HealthCustom, func(
		_ context.Context, target HealthTarget,
	) *HealthResult {
		return &HealthResult{
			Target:    target.Name,
			Healthy:   true,
			Timestamp: time.Now(),
		}
	})

	targets := []HealthTarget{
		{Name: "first", Type: HealthCustom},
		{Name: "second", Type: HealthCustom},
		{Name: "third", Type: HealthCustom},
	}

	ctx := context.Background()
	all := c.CheckAll(ctx, targets)
	require.Len(t, all, 3)
	assert.Equal(t, "first", all[0].Target)
	assert.Equal(t, "second", all[1].Target)
	assert.Equal(t, "third", all[2].Target)
}
