package health

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCustomCheckFunc_Success(t *testing.T) {
	customFn := func(_ context.Context, _ HealthTarget) error {
		return nil
	}

	checkFunc := NewCustomCheckFunc(customFn)
	target := HealthTarget{Name: "custom-test", Type: HealthCustom}
	ctx := context.Background()

	result := checkFunc(ctx, target)

	assert.True(t, result.Healthy)
	assert.Equal(t, "custom-test", result.Target)
	assert.Empty(t, result.Error)
	assert.Equal(t, "custom", result.Details["type"])
	assert.NotZero(t, result.Duration)
	assert.False(t, result.Timestamp.IsZero())
}

func TestNewCustomCheckFunc_Failure(t *testing.T) {
	customFn := func(_ context.Context, _ HealthTarget) error {
		return fmt.Errorf("service unavailable")
	}

	checkFunc := NewCustomCheckFunc(customFn)
	target := HealthTarget{Name: "custom-fail", Type: HealthCustom}
	ctx := context.Background()

	result := checkFunc(ctx, target)

	assert.False(t, result.Healthy)
	assert.Equal(t, "custom-fail", result.Target)
	assert.Contains(t, result.Error, "custom check failed")
	assert.Contains(t, result.Error, "service unavailable")
	assert.Equal(t, "custom", result.Details["type"])
}

func TestNewCustomCheckFunc_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		healthy bool
		errMsg  string
	}{
		{
			name:    "healthy service",
			err:     nil,
			healthy: true,
		},
		{
			name:    "timeout error",
			err:     fmt.Errorf("connection timeout"),
			healthy: false,
			errMsg:  "connection timeout",
		},
		{
			name:    "auth error",
			err:     fmt.Errorf("authentication failed"),
			healthy: false,
			errMsg:  "authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customFn := func(_ context.Context, _ HealthTarget) error {
				return tt.err
			}

			checkFunc := NewCustomCheckFunc(customFn)
			target := HealthTarget{Name: tt.name, Type: HealthCustom}
			ctx := context.Background()

			result := checkFunc(ctx, target)

			assert.Equal(t, tt.healthy, result.Healthy)
			assert.Equal(t, tt.name, result.Target)
			if tt.errMsg != "" {
				assert.Contains(t, result.Error, tt.errMsg)
			}
		})
	}
}

func TestNewCustomCheckFunc_MeasuresDuration(t *testing.T) {
	delay := 50 * time.Millisecond
	customFn := func(_ context.Context, _ HealthTarget) error {
		time.Sleep(delay)
		return nil
	}

	checkFunc := NewCustomCheckFunc(customFn)
	target := HealthTarget{Name: "duration-test", Type: HealthCustom}
	ctx := context.Background()

	result := checkFunc(ctx, target)

	assert.True(t, result.Healthy)
	assert.GreaterOrEqual(t, result.Duration, delay)
}

func TestNewCustomCheckFunc_RegisterWithChecker(t *testing.T) {
	checker := NewDefaultChecker()
	customFn := func(_ context.Context, _ HealthTarget) error {
		return nil
	}

	checker.Register(HealthCustom, NewCustomCheckFunc(customFn))

	target := HealthTarget{Name: "registered-custom", Type: HealthCustom}
	ctx := context.Background()

	result := checker.Check(ctx, target)

	assert.True(t, result.Healthy)
	assert.Equal(t, "custom", result.Details["type"])
}
