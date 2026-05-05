package orchestrator

import (
	"context"
	"testing"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
	"github.com/stretchr/testify/assert"
)

type mockComposeOrch struct{}

func (m *mockComposeOrch) Up(ctx context.Context, p compose.ComposeProject) error   { return nil }
func (m *mockComposeOrch) Down(ctx context.Context, p compose.ComposeProject) error { return nil }

type mockRemoteExec struct{}

func (m *mockRemoteExec) Execute(ctx context.Context, host remote.RemoteHost, cmd string) (*remote.CommandResult, error) {
	return nil, nil
}
func (m *mockRemoteExec) CopyDir(ctx context.Context, host remote.RemoteHost, src, dst string) error {
	return nil
}

type mockHostMgr struct{}

func (m *mockHostMgr) ListHosts() []remote.RemoteHost { return nil }

type mockHealthChecker struct{}

func (m *mockHealthChecker) Check(ctx context.Context, target health.HealthTarget) *health.HealthResult {
	return &health.HealthResult{Healthy: true}
}
func (m *mockHealthChecker) CheckAll(ctx context.Context, targets []health.HealthTarget) []*health.HealthResult {
	return nil
}

func TestWithLocalOrchestrator(t *testing.T) {
	o := &DefaultOrchestrator{}
	mock := &mockComposeOrch{}
	WithLocalOrchestrator(mock)(o)
	assert.Equal(t, mock, o.localOrch)
}

func TestWithRemoteExecutor(t *testing.T) {
	o := &DefaultOrchestrator{}
	mock := &mockRemoteExec{}
	WithRemoteExecutor(mock)(o)
	assert.Equal(t, mock, o.remoteExec)
}

func TestWithHostManagerOrchestrator(t *testing.T) {
	o := &DefaultOrchestrator{}
	mock := &mockHostMgr{}
	WithHostManager(mock)(o)
	assert.Equal(t, mock, o.hostMgr)
}

func TestWithHealthCheckerOrchestrator(t *testing.T) {
	o := &DefaultOrchestrator{}
	mock := &mockHealthChecker{}
	WithHealthChecker(mock)(o)
	assert.Equal(t, mock, o.healthChecker)
}

func TestWithLoggerOrchestrator(t *testing.T) {
	o := &DefaultOrchestrator{}
	l := logging.NopLogger{}
	WithLogger(l)(o)
	assert.NotNil(t, o.logger)
}

func TestWithProjectDirOrchestrator(t *testing.T) {
	o := &DefaultOrchestrator{}
	WithProjectDir("/my/project")(o)
	assert.Equal(t, "/my/project", o.projectDir)
}

func TestWithExcludePattern(t *testing.T) {
	o := &DefaultOrchestrator{}
	WithExcludePattern("*test*")(o)
	assert.Equal(t, "*test*", o.excludePattern)
}

func TestNew_RemoteEnabled(t *testing.T) {
	execMock := &mockRemoteExec{}
	hostMock := &mockHostMgr{}
	orch := New(
		WithRemoteExecutor(execMock),
		WithHostManager(hostMock),
	)
	assert.True(t, orch.remoteEnabled)
}

func TestNew_RemoteDisabled_WhenOnlyExecutor(t *testing.T) {
	execMock := &mockRemoteExec{}
	orch := New(WithRemoteExecutor(execMock))
	assert.False(t, orch.remoteEnabled)
}
