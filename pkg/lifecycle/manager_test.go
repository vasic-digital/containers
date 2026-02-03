package lifecycle_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/lifecycle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOrchestrator records calls and returns preset errors.
type stubOrchestrator struct {
	upCalled   int
	downCalled int
	upErr      error
	downErr    error
}

func (s *stubOrchestrator) Up(
	_ context.Context, _ compose.ComposeProject,
	_ ...compose.UpOption,
) error {
	s.upCalled++
	return s.upErr
}
func (s *stubOrchestrator) Down(
	_ context.Context, _ compose.ComposeProject,
	_ ...compose.DownOption,
) error {
	s.downCalled++
	return s.downErr
}
func (s *stubOrchestrator) Status(
	_ context.Context, _ compose.ComposeProject,
) ([]compose.ServiceStatus, error) {
	return nil, nil
}
func (s *stubOrchestrator) Logs(
	_ context.Context, _ compose.ComposeProject, _ string,
) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}

// Verify interface compliance.
var _ compose.ComposeOrchestrator = (*stubOrchestrator)(nil)

// stubHealthChecker returns a preset result.
type stubHealthChecker struct {
	healthy bool
	errMsg  string
}

func (c *stubHealthChecker) Check(
	_ context.Context, t health.HealthTarget,
) *health.HealthResult {
	r := &health.HealthResult{
		Target:    t.Name,
		Healthy:   c.healthy,
		Timestamp: time.Now(),
	}
	if !c.healthy && c.errMsg != "" {
		r.Error = c.errMsg
	}
	return r
}

func (c *stubHealthChecker) CheckAll(
	_ context.Context, targets []health.HealthTarget,
) []*health.HealthResult {
	results := make([]*health.HealthResult, len(targets))
	for i, t := range targets {
		results[i] = c.Check(context.Background(), t)
	}
	return results
}

var _ health.HealthChecker = (*stubHealthChecker)(nil)

func TestDefaultManager_Register(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)

	err := m.Register(lifecycle.ServiceSpec{Name: "redis"})
	require.NoError(t, err)

	// Duplicate registration fails.
	err = m.Register(lifecycle.ServiceSpec{Name: "redis"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestDefaultManager_Register_EmptyName(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	err := m.Register(lifecycle.ServiceSpec{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestDefaultManager_StartStop(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "pg",
		ComposeFile: "docker-compose.yml",
		Profile:     "core",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "pg"))
	assert.Equal(t, 1, orch.upCalled)

	status, err := m.Status("pg")
	require.NoError(t, err)
	assert.Equal(t, "running", status.State)
	assert.True(t, status.Healthy)

	require.NoError(t, m.Stop(ctx, "pg"))
	assert.Equal(t, 1, orch.downCalled)

	status, err = m.Status("pg")
	require.NoError(t, err)
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Start_NotFound(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	err := m.Start(context.Background(), "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultManager_Stop_NotFound(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	err := m.Stop(context.Background(), "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultManager_Status_NotFound(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	_, err := m.Status("missing")
	assert.Error(t, err)
}

func TestDefaultManager_Start_AlreadyRunning(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "redis",
		ComposeFile: "compose.yml",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "redis"))
	require.NoError(t, m.Start(ctx, "redis")) // idempotent
	assert.Equal(t, 1, orch.upCalled)
}

func TestDefaultManager_Stop_AlreadyStopped(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name: "svc",
	}))
	require.NoError(t, m.Stop(context.Background(), "svc"))
}

func TestDefaultManager_Acquire_NotRunning(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name: "svc",
	}))

	_, err := m.Acquire(context.Background(), "svc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestDefaultManager_Acquire_NotFound(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)
	_, err := m.Acquire(context.Background(), "nope")
	assert.Error(t, err)
}

func TestDefaultManager_Acquire_Running(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:          "svc",
		ComposeFile:   "c.yml",
		MaxConcurrent: 2,
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	release, err := m.Acquire(ctx, "svc")
	require.NoError(t, err)
	require.NotNil(t, release)

	status, _ := m.Status("svc")
	assert.Equal(t, 1, status.ActiveUsers)

	release()
	status, _ = m.Status("svc")
	assert.Equal(t, 0, status.ActiveUsers)
}

func TestDefaultManager_Acquire_LazyBoot(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "lazy",
		LazyBoot:    true,
		ComposeFile: "c.yml",
	}))

	ctx := context.Background()
	release, err := m.Acquire(ctx, "lazy")
	require.NoError(t, err)
	require.NotNil(t, release)
	assert.Equal(t, 1, orch.upCalled)

	release()
}

func TestDefaultManager_Shutdown(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "a",
		ComposeFile: "a.yml",
	}))
	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "b",
		ComposeFile: "b.yml",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "a"))
	require.NoError(t, m.Start(ctx, "b"))

	require.NoError(t, m.Shutdown(ctx))
	assert.Equal(t, 2, orch.downCalled)
}

func TestDefaultManager_Dependencies(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "db",
		ComposeFile: "db.yml",
	}))
	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:         "app",
		ComposeFile:  "app.yml",
		Dependencies: []string{"db"},
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "app"))
	// Both should have been started.
	assert.Equal(t, 2, orch.upCalled)
}

// Additional tests for missing coverage

func TestDefaultManager_Start_ComposeUpError(t *testing.T) {
	orch := &stubOrchestrator{
		upErr: errors.New("compose up failed"),
	}
	m := lifecycle.NewDefaultManager(orch, nil)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
	}))

	err := m.Start(context.Background(), "svc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compose up failed")

	// State should remain stopped.
	status, _ := m.Status("svc")
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Start_HealthCheckFailure(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: false, errMsg: "connection refused"}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
	}))

	err := m.Start(context.Background(), "svc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check")
	assert.Contains(t, err.Error(), "connection refused")

	// State should be stopped after health check failure.
	status, _ := m.Status("svc")
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Start_DependencyError(t *testing.T) {
	orch := &stubOrchestrator{}
	m := lifecycle.NewDefaultManager(orch, nil)

	// Register only the dependent, not the dependency.
	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:         "app",
		ComposeFile:  "app.yml",
		Dependencies: []string{"missing-db"},
	}))

	err := m.Start(context.Background(), "app")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency")
	assert.Contains(t, err.Error(), "missing-db")
}

func TestDefaultManager_Stop_ComposeDownError(t *testing.T) {
	orch := &stubOrchestrator{
		downErr: errors.New("compose down failed"),
	}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	err := m.Stop(ctx, "svc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compose down failed")

	// State should revert to running on error.
	status, _ := m.Status("svc")
	assert.Equal(t, "running", status.State)
}

func TestDefaultManager_Acquire_SemaphoreTimeout(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:          "svc",
		ComposeFile:   "c.yml",
		MaxConcurrent: 1, // Only 1 concurrent allowed.
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	// First acquire succeeds.
	release1, err := m.Acquire(ctx, "svc")
	require.NoError(t, err)

	// Second acquire with timeout should fail.
	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_, err = m.Acquire(timeoutCtx, "svc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "semaphore")

	release1()
}

func TestDefaultManager_Acquire_LazyBootError(t *testing.T) {
	orch := &stubOrchestrator{
		upErr: errors.New("lazy boot failed"),
	}
	m := lifecycle.NewDefaultManager(orch, nil)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "lazy",
		LazyBoot:    true,
		ComposeFile: "c.yml",
	}))

	_, err := m.Acquire(context.Background(), "lazy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lazy boot")
}

func TestDefaultManager_Shutdown_WithError(t *testing.T) {
	orch := &stubOrchestrator{
		downErr: errors.New("shutdown error"),
	}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "a",
		ComposeFile: "a.yml",
	}))
	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "b",
		ComposeFile: "b.yml",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "a"))
	require.NoError(t, m.Start(ctx, "b"))

	err := m.Shutdown(ctx)
	// Should return the first error.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown error")
}

func TestDefaultManager_Start_WithIdleTimeout(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
		IdleTimeout: 200 * time.Millisecond,
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	// Wait for idle timeout to trigger stop.
	time.Sleep(300 * time.Millisecond)

	// Service should have been stopped by idle shutdown.
	status, err := m.Status("svc")
	require.NoError(t, err)
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Acquire_WithIdleShutdown(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:          "svc",
		ComposeFile:   "c.yml",
		IdleTimeout:   500 * time.Millisecond,
		MaxConcurrent: 2,
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	// Acquire to touch idle timer.
	release, err := m.Acquire(ctx, "svc")
	require.NoError(t, err)

	// Release should touch idle timer again.
	release()

	// Calling release again (idempotent) should be safe.
	release()

	status, _ := m.Status("svc")
	assert.Equal(t, 0, status.ActiveUsers)
}

func TestDefaultManager_Stop_WithRunningIdleController(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
		IdleTimeout: time.Hour, // Long timeout, won't trigger.
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	// Stop should stop the idle controller.
	require.NoError(t, m.Stop(ctx, "svc"))

	status, _ := m.Status("svc")
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Start_NoComposeFile(t *testing.T) {
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(nil, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name: "svc",
		// No ComposeFile, no orchestrator call.
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	status, _ := m.Status("svc")
	assert.Equal(t, "running", status.State)
}

func TestDefaultManager_Stop_NoComposeFile(t *testing.T) {
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(nil, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name: "svc",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))
	require.NoError(t, m.Stop(ctx, "svc"))

	status, _ := m.Status("svc")
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Start_NilOrchestrator(t *testing.T) {
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(nil, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml", // Has compose file but nil orchestrator.
	}))

	ctx := context.Background()
	// Should succeed even with nil orchestrator (skips compose up).
	require.NoError(t, m.Start(ctx, "svc"))

	status, _ := m.Status("svc")
	assert.Equal(t, "running", status.State)
}

func TestDefaultManager_Acquire_NoSemaphore(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
		// MaxConcurrent = 0 means no semaphore.
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	release, err := m.Acquire(ctx, "svc")
	require.NoError(t, err)
	require.NotNil(t, release)

	// ActiveUsers should be 0 since no semaphore.
	status, _ := m.Status("svc")
	assert.Equal(t, 0, status.ActiveUsers)

	release()
}

func TestDefaultManager_Register_WithMaxConcurrent(t *testing.T) {
	m := lifecycle.NewDefaultManager(nil, nil)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:          "svc",
		MaxConcurrent: 5,
	}))

	// Verify registration succeeded.
	status, err := m.Status("svc")
	require.NoError(t, err)
	assert.Equal(t, "stopped", status.State)
}

func TestDefaultManager_Start_NilHealthChecker(t *testing.T) {
	orch := &stubOrchestrator{}
	m := lifecycle.NewDefaultManager(orch, nil)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "svc",
		ComposeFile: "c.yml",
	}))

	ctx := context.Background()
	require.NoError(t, m.Start(ctx, "svc"))

	status, _ := m.Status("svc")
	assert.Equal(t, "running", status.State)
	// Healthy should be false (no health checker, never set).
	assert.False(t, status.Healthy)
}

func TestDefaultManager_Acquire_LazyBoot_MultipleAcquires(t *testing.T) {
	orch := &stubOrchestrator{}
	hc := &stubHealthChecker{healthy: true}
	m := lifecycle.NewDefaultManager(orch, hc)

	require.NoError(t, m.Register(lifecycle.ServiceSpec{
		Name:        "lazy",
		LazyBoot:    true,
		ComposeFile: "c.yml",
	}))

	ctx := context.Background()

	// First acquire triggers lazy boot.
	release1, err := m.Acquire(ctx, "lazy")
	require.NoError(t, err)
	assert.Equal(t, 1, orch.upCalled)

	// Second acquire should not trigger another start.
	release2, err := m.Acquire(ctx, "lazy")
	require.NoError(t, err)
	assert.Equal(t, 1, orch.upCalled) // Still 1.

	release1()
	release2()
}
