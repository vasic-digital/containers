package boot_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/discovery"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test doubles ---

type mockOrchestrator struct {
	upCalled   int
	downCalled int
	upErr      error
	downErr    error
}

func (m *mockOrchestrator) Up(
	_ context.Context, _ compose.ComposeProject,
	_ ...compose.UpOption,
) error {
	m.upCalled++
	return m.upErr
}
func (m *mockOrchestrator) Down(
	_ context.Context, _ compose.ComposeProject,
	_ ...compose.DownOption,
) error {
	m.downCalled++
	return m.downErr
}
func (m *mockOrchestrator) Status(
	_ context.Context, _ compose.ComposeProject,
) ([]compose.ServiceStatus, error) {
	return nil, nil
}
func (m *mockOrchestrator) Logs(
	_ context.Context, _ compose.ComposeProject, _ string,
) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}

var _ compose.ComposeOrchestrator = (*mockOrchestrator)(nil)

type mockHealthChecker struct {
	results map[string]bool
}

func (m *mockHealthChecker) Check(
	_ context.Context, t health.HealthTarget,
) *health.HealthResult {
	h := true
	if m.results != nil {
		if v, ok := m.results[t.Name]; ok {
			h = v
		}
	}
	r := &health.HealthResult{
		Target:    t.Name,
		Healthy:   h,
		Timestamp: time.Now(),
	}
	if !h {
		r.Error = "unhealthy"
	}
	return r
}

func (m *mockHealthChecker) CheckAll(
	ctx context.Context, targets []health.HealthTarget,
) []*health.HealthResult {
	out := make([]*health.HealthResult, len(targets))
	for i, t := range targets {
		out[i] = m.Check(ctx, t)
	}
	return out
}

var _ health.HealthChecker = (*mockHealthChecker)(nil)

type mockDiscoverer struct {
	found map[string]bool
}

func (m *mockDiscoverer) Discover(
	_ context.Context, t discovery.DiscoveryTarget,
) (bool, error) {
	if m.found != nil {
		if v, ok := m.found[t.Name]; ok {
			return v, nil
		}
	}
	return false, errors.New("not discovered")
}

var _ discovery.Discoverer = (*mockDiscoverer)(nil)

// --- Tests ---

func TestBootManager_BootAll_BasicSuccess(t *testing.T) {
	orch := &mockOrchestrator{}
	hc := &mockHealthChecker{
		results: map[string]bool{"redis": true, "pg": true},
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"redis": {
			Host:        "localhost",
			Port:        "6379",
			Enabled:     true,
			Required:    true,
			ComposeFile: "docker-compose.yml",
			HealthType:  "tcp",
			Timeout:     5 * time.Second,
		},
		"pg": {
			Host:        "localhost",
			Port:        "5432",
			Enabled:     true,
			Required:    true,
			ComposeFile: "docker-compose.yml",
			HealthType:  "tcp",
			Timeout:     5 * time.Second,
		},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithOrchestrator(orch),
		boot.WithHealthChecker(hc),
		boot.WithLogger(logging.NewStdLogger("test")),
	)

	summary, err := bm.BootAll(context.Background())
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 2, summary.Started)
	assert.Equal(t, 0, summary.Failed)
	assert.Equal(t, 1, orch.upCalled) // grouped by compose file
}

func TestBootManager_BootAll_DisabledSkipped(t *testing.T) {
	endpoints := map[string]endpoint.ServiceEndpoint{
		"disabled": {
			Enabled: false,
		},
	}

	bm := boot.NewBootManager(endpoints)
	summary, err := bm.BootAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Skipped)
}

func TestBootManager_BootAll_DiscoveryPhase(t *testing.T) {
	disc := &mockDiscoverer{
		found: map[string]bool{"remote-db": true},
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"remote-db": {
			Host:             "db.example.com",
			Port:             "5432",
			Enabled:          true,
			DiscoveryEnabled: true,
			DiscoveryMethod:  "tcp",
			DiscoveryTimeout: time.Second,
		},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithDiscoverer(disc),
	)

	summary, err := bm.BootAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Discovered)
}

func TestBootManager_BootAll_ComposeFailure(t *testing.T) {
	orch := &mockOrchestrator{
		upErr: errors.New("compose up failed"),
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"svc": {
			Host:        "localhost",
			Port:        "8080",
			Enabled:     true,
			Required:    true,
			ComposeFile: "c.yml",
		},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithOrchestrator(orch),
	)

	summary, err := bm.BootAll(context.Background())
	assert.Error(t, err)
	assert.Equal(t, 1, summary.Failed)
}

func TestBootManager_BootAll_HealthFailRequired(t *testing.T) {
	orch := &mockOrchestrator{}
	hc := &mockHealthChecker{
		results: map[string]bool{"svc": false},
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"svc": {
			Host:        "localhost",
			Port:        "8080",
			Enabled:     true,
			Required:    true,
			ComposeFile: "c.yml",
			HealthType:  "tcp",
		},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithOrchestrator(orch),
		boot.WithHealthChecker(hc),
	)

	summary, err := bm.BootAll(context.Background())
	assert.Error(t, err)
	assert.True(t, summary.HasFailures())
}

func TestBootManager_BootAll_RemoteService(t *testing.T) {
	hc := &mockHealthChecker{
		results: map[string]bool{"remote": true},
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"remote": {
			Host:        "remote.host",
			Port:        "443",
			Enabled:     true,
			Remote:      true,
			ComposeFile: "c.yml",
			HealthType:  "http",
		},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithOrchestrator(&mockOrchestrator{}),
		boot.WithHealthChecker(hc),
	)

	summary, err := bm.BootAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Remote)
}

func TestBootManager_HealthCheckAll(t *testing.T) {
	hc := &mockHealthChecker{
		results: map[string]bool{
			"ok":   true,
			"fail": false,
		},
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"ok":   {Enabled: true, HealthType: "tcp"},
		"fail": {Enabled: true, HealthType: "tcp"},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithHealthChecker(hc),
	)

	errs := bm.HealthCheckAll(context.Background())
	assert.Nil(t, errs["ok"])
	assert.NotNil(t, errs["fail"])
}

func TestBootManager_HealthCheckAll_NoChecker(t *testing.T) {
	bm := boot.NewBootManager(
		map[string]endpoint.ServiceEndpoint{},
	)
	errs := bm.HealthCheckAll(context.Background())
	assert.Empty(t, errs)
}

func TestBootManager_Shutdown(t *testing.T) {
	orch := &mockOrchestrator{}
	endpoints := map[string]endpoint.ServiceEndpoint{
		"svc": {
			Enabled:     true,
			ComposeFile: "c.yml",
			Profile:     "core",
		},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithOrchestrator(orch),
	)

	err := bm.Shutdown(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, orch.downCalled)
}

func TestBootManager_Shutdown_Error(t *testing.T) {
	orch := &mockOrchestrator{
		downErr: errors.New("down failed"),
	}
	endpoints := map[string]endpoint.ServiceEndpoint{
		"svc": {Enabled: true, ComposeFile: "c.yml"},
	}

	bm := boot.NewBootManager(endpoints,
		boot.WithOrchestrator(orch),
	)

	err := bm.Shutdown(context.Background())
	assert.Error(t, err)
}
