package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockComposeOrchestrator struct {
	upCalled   bool
	downCalled bool
	upError    error
	downError  error
}

func (m *mockComposeOrchestrator) Up(ctx context.Context, project compose.ComposeProject) error {
	m.upCalled = true
	return m.upError
}

func (m *mockComposeOrchestrator) Down(ctx context.Context, project compose.ComposeProject) error {
	m.downCalled = true
	return m.downError
}

type mockRemoteExecutor struct {
	executeCalled bool
	copyDirCalled bool
	lastCommand   string
	lastSrcDir    string
	lastDstDir    string
	executeResult *remote.CommandResult
	executeError  error
	copyDirError  error
}

func (m *mockRemoteExecutor) Execute(ctx context.Context, host remote.RemoteHost, cmd string) (*remote.CommandResult, error) {
	m.executeCalled = true
	m.lastCommand = cmd
	if m.executeResult != nil {
		return m.executeResult, m.executeError
	}
	return &remote.CommandResult{Stdout: "", Stderr: "", ExitCode: 0}, m.executeError
}

func (m *mockRemoteExecutor) CopyDir(ctx context.Context, host remote.RemoteHost, src, dst string) error {
	m.copyDirCalled = true
	m.lastSrcDir = src
	m.lastDstDir = dst
	return m.copyDirError
}

type mockHostManager struct {
	hosts []remote.RemoteHost
}

func (m *mockHostManager) ListHosts() []remote.RemoteHost {
	return m.hosts
}

func TestNewOrchestrator(t *testing.T) {
	t.Run("creates orchestrator with defaults", func(t *testing.T) {
		orch := New()
		require.NotNil(t, orch)
		assert.False(t, orch.remoteEnabled)
		assert.Equal(t, 0, orch.ServiceCount())
	})

	t.Run("creates orchestrator with options", func(t *testing.T) {
		mockOrch := &mockComposeOrchestrator{}
		orch := New(
			WithLocalOrchestrator(mockOrch),
			WithProjectDir("/test"),
		)
		require.NotNil(t, orch)
		assert.Equal(t, "/test", orch.projectDir)
	})
}

func TestAddService(t *testing.T) {
	orch := New()
	orch.AddService(Service{
		Name:        "test-service",
		ComposeFile: "docker/test/docker-compose.yml",
	})

	services := orch.ListServices()
	require.Len(t, services, 1)
	assert.Equal(t, "test-service", services[0].Name)
	assert.Equal(t, "docker/test/docker-compose.yml", services[0].ComposeFile)
}

func TestListServices(t *testing.T) {
	orch := New()
	orch.AddService(Service{Name: "service1"})
	orch.AddService(Service{Name: "service2"})

	services := orch.ListServices()
	require.Len(t, services, 2)

	services[0].Name = "modified"
	servicesAgain := orch.ListServices()
	assert.Equal(t, "service1", servicesAgain[0].Name, "ListServices should return a copy")
}

func TestIsRemoteEnabled(t *testing.T) {
	t.Run("disabled without remote config", func(t *testing.T) {
		orch := New()
		assert.False(t, orch.IsRemoteEnabled())
	})

	t.Run("enabled with remote config", func(t *testing.T) {
		mockExec := &mockRemoteExecutor{}
		mockMgr := &mockHostManager{hosts: []remote.RemoteHost{{Name: "test"}}}
		orch := New(
			WithRemoteExecutor(mockExec),
			WithHostManager(mockMgr),
		)
		assert.True(t, orch.IsRemoteEnabled())
	})
}

func TestStartAll(t *testing.T) {
	t.Run("skips non-existent compose files", func(t *testing.T) {
		orch := New()
		orch.AddService(Service{
			Name:        "nonexistent",
			ComposeFile: "/nonexistent/path/docker-compose.yml",
		})

		err := orch.StartAll(context.Background())
		assert.NoError(t, err)
	})

	t.Run("starts services with local orchestrator", func(t *testing.T) {
		mockOrch := &mockComposeOrchestrator{}
		orch := New(WithLocalOrchestrator(mockOrch))

		orch.AddService(Service{
			Name:        "test",
			ComposeFile: "orchestrator_test.go",
		})

		err := orch.StartAll(context.Background())
		assert.NoError(t, err)
	})
}

func TestRemoteStart(t *testing.T) {
	t.Run("succeeds with proper remote configuration", func(t *testing.T) {
		mockExec := &mockRemoteExecutor{
			executeResult: &remote.CommandResult{ExitCode: 0},
		}
		mockMgr := &mockHostManager{
			hosts: []remote.RemoteHost{{Name: "test", User: "testuser"}},
		}
		orch := New(
			WithRemoteExecutor(mockExec),
			WithHostManager(mockMgr),
		)

		// Use a file that exists in the projectDir (test package directory)
		orch.AddService(Service{
			Name:        "test",
			ComposeFile: "orchestrator_test.go",
		})

		err := orch.StartAll(context.Background())
		assert.NoError(t, err)
		assert.True(t, mockExec.executeCalled, "remote Execute should be called")
		assert.True(t, mockExec.copyDirCalled, "remote CopyDir should be called")
	})

	t.Run("uses remote executor when configured", func(t *testing.T) {
		mockExec := &mockRemoteExecutor{
			executeResult: &remote.CommandResult{ExitCode: 0},
		}
		mockMgr := &mockHostManager{
			hosts: []remote.RemoteHost{
				{Name: "test", User: "testuser"},
			},
		}
		orch := New(
			WithRemoteExecutor(mockExec),
			WithHostManager(mockMgr),
		)

		assert.True(t, orch.IsRemoteEnabled())
	})

	t.Run("returns error when no hosts available", func(t *testing.T) {
		mockExec := &mockRemoteExecutor{
			executeResult: &remote.CommandResult{ExitCode: 0},
		}
		mockMgr := &mockHostManager{
			hosts: []remote.RemoteHost{},
		}
		mockLocalOrch := &mockComposeOrchestrator{upError: fmt.Errorf("local failed")}
		orch := New(
			WithRemoteExecutor(mockExec),
			WithHostManager(mockMgr),
			WithLocalOrchestrator(mockLocalOrch),
		)

		orch.AddService(Service{
			Name:        "test",
			ComposeFile: "orchestrator_test.go",
			Required:    true,
		})

		err := orch.StartAll(context.Background())
		assert.Error(t, err)
	})
}
