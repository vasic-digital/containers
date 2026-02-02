package lifecycle_test

import (
	"context"
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
}

func (c *stubHealthChecker) Check(
	_ context.Context, t health.HealthTarget,
) *health.HealthResult {
	return &health.HealthResult{
		Target:    t.Name,
		Healthy:   c.healthy,
		Timestamp: time.Now(),
	}
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
