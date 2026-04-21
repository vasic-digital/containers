package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/remote"
)

// --- DiscoverServices additional branch coverage ---

// TestDiscoverServices_DirNotFound exercises the os.IsNotExist branch when
// the directory does not exist.
func TestDiscoverServices_DirNotFound(t *testing.T) {
	o := New()
	err := o.DiscoverServices("/nonexistent/does/not/exist/dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestDiscoverServices_WithFiles creates a temporary directory containing
// a docker-compose.yml and asserts that the service is discovered.
func TestDiscoverServices_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myservice")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	composeFile := filepath.Join(subDir, "docker-compose.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir))
	err := o.DiscoverServices(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, o.services, 1)
	assert.Equal(t, "myservice", o.services[0].Name)
}

// TestDiscoverServices_RelativePath exercises the relative-to-projectDir
// join branch in DiscoverServices.
func TestDiscoverServices_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "relativeservice")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	composeFile := filepath.Join(subDir, "docker-compose.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir))
	// Pass the base name only — relative to projectDir.
	err := o.DiscoverServices("relativeservice")
	assert.NoError(t, err)
	// The single compose file should be discovered.
	assert.Len(t, o.services, 1)
}

// TestDiscoverServices_ExcludePattern exercises the excludePattern branch:
// files matching the pattern are skipped.
func TestDiscoverServices_ExcludePattern(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "excluded")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	// This file name matches the exclude pattern.
	composeFile := filepath.Join(subDir, "docker-compose.test.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir), WithExcludePattern("docker-compose.test.yml"))
	err := o.DiscoverServices(tmpDir)
	assert.NoError(t, err)
	// The matching file is excluded, so no services should be discovered.
	assert.Empty(t, o.services)
}

// TestDiscoverServices_DuplicateFileNotAdded ensures that the same compose
// file is not added twice when DiscoverServices is called more than once.
func TestDiscoverServices_DuplicateFileNotAdded(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "dup")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	composeFile := filepath.Join(subDir, "docker-compose.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir))
	require.NoError(t, o.DiscoverServices(tmpDir))
	assert.Len(t, o.services, 1)

	// Call again — the service should not be duplicated.
	require.NoError(t, o.DiscoverServices(tmpDir))
	assert.Len(t, o.services, 1)
}

// --- startRemote branch coverage ---

// moreTestRemoteExec is a minimal RemoteExecutor for startRemote tests.
type moreTestRemoteExec struct {
	executeFunc func(ctx context.Context, host remote.RemoteHost, cmd string) (*remote.CommandResult, error)
	copyDirFunc func(ctx context.Context, host remote.RemoteHost, src, dst string) error
}

func (m *moreTestRemoteExec) Execute(ctx context.Context, host remote.RemoteHost, cmd string) (*remote.CommandResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, host, cmd)
	}
	return &remote.CommandResult{ExitCode: 0}, nil
}

func (m *moreTestRemoteExec) CopyDir(ctx context.Context, host remote.RemoteHost, src, dst string) error {
	if m.copyDirFunc != nil {
		return m.copyDirFunc(ctx, host, src, dst)
	}
	return nil
}

// moreTestHostMgr is a minimal HostManager for startRemote tests.
type moreTestHostMgr struct {
	hosts []remote.RemoteHost
}

func (m *moreTestHostMgr) ListHosts() []remote.RemoteHost {
	return m.hosts
}

// moreComposeOrch is a minimal ComposeOrchestrator satisfying the local
// orchestrator.ComposeOrchestrator interface (Up and Down without variadic).
type moreComposeOrch struct {
	downErr    error
	downCalled bool
}

func (m *moreComposeOrch) Up(_ context.Context, _ compose.ComposeProject) error {
	return nil
}

func (m *moreComposeOrch) Down(_ context.Context, _ compose.ComposeProject) error {
	m.downCalled = true
	return m.downErr
}

// TestStartRemote_NoHosts exercises the "no remote hosts available" branch
// in startRemote when the host manager returns an empty slice.
func TestStartRemote_NoHosts(t *testing.T) {
	hostMgr := &moreTestHostMgr{hosts: []remote.RemoteHost{}}
	exec := &moreTestRemoteExec{}

	o := New(
		WithRemoteExecutor(exec),
		WithHostManager(hostMgr),
	)
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no remote hosts")
}

// TestStartRemote_MkdirFail exercises the branch where the mkdir Execute
// call returns a non-nil error.
func TestStartRemote_MkdirFail(t *testing.T) {
	hostMgr := &moreTestHostMgr{
		hosts: []remote.RemoteHost{{Name: "h1", Address: "10.0.0.1", User: "u"}},
	}

	exec := &moreTestRemoteExec{
		executeFunc: func(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
			// First call is mkdir — return an error.
			return nil, fmt.Errorf("ssh connection refused")
		},
	}

	o := New(WithRemoteExecutor(exec), WithHostManager(hostMgr))
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create remote dir")
}

// TestStartRemote_MkdirNonZeroExit exercises the branch where mkdir succeeds
// at the transport level but exits with a non-zero code.
func TestStartRemote_MkdirNonZeroExit(t *testing.T) {
	hostMgr := &moreTestHostMgr{
		hosts: []remote.RemoteHost{{Name: "h1", Address: "10.0.0.1", User: "u"}},
	}
	exec := &moreTestRemoteExec{
		executeFunc: func(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
			// First call is mkdir — non-zero exit.
			return &remote.CommandResult{ExitCode: 1, Stderr: "permission denied"}, nil
		},
	}

	o := New(WithRemoteExecutor(exec), WithHostManager(hostMgr))
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create remote dir failed")
}

// TestStartRemote_CopyDirFail exercises the CopyDir error branch.
func TestStartRemote_CopyDirFail(t *testing.T) {
	hostMgr := &moreTestHostMgr{
		hosts: []remote.RemoteHost{{Name: "h1", Address: "10.0.0.1", User: "u"}},
	}
	exec := &moreTestRemoteExec{
		executeFunc: func(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
			// mkdir succeeds.
			return &remote.CommandResult{ExitCode: 0}, nil
		},
		copyDirFunc: func(_ context.Context, _ remote.RemoteHost, _, _ string) error {
			return fmt.Errorf("scp failed: no route to host")
		},
	}

	o := New(WithRemoteExecutor(exec), WithHostManager(hostMgr))
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "copy to remote")
}

// TestStartRemote_ComposeExecFail exercises the branch where the compose
// Execute call returns a non-nil error.
func TestStartRemote_ComposeExecFail(t *testing.T) {
	hostMgr := &moreTestHostMgr{
		hosts: []remote.RemoteHost{{Name: "h1", Address: "10.0.0.1", User: "u"}},
	}
	callCount := 0
	exec := &moreTestRemoteExec{
		executeFunc: func(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
			callCount++
			if callCount == 1 {
				// mkdir succeeds.
				return &remote.CommandResult{ExitCode: 0}, nil
			}
			// compose exec fails.
			return nil, fmt.Errorf("docker: daemon not running")
		},
		copyDirFunc: func(_ context.Context, _ remote.RemoteHost, _, _ string) error {
			return nil // copy succeeds
		},
	}

	o := New(WithRemoteExecutor(exec), WithHostManager(hostMgr))
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	require.Error(t, err)
}

// TestStartRemote_ComposeNonZeroExit exercises the branch where the compose
// command returns exit code 1.
func TestStartRemote_ComposeNonZeroExit(t *testing.T) {
	hostMgr := &moreTestHostMgr{
		hosts: []remote.RemoteHost{{Name: "h1", Address: "10.0.0.1", User: "u"}},
	}
	callCount := 0
	exec := &moreTestRemoteExec{
		executeFunc: func(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
			callCount++
			if callCount == 1 {
				// mkdir succeeds.
				return &remote.CommandResult{ExitCode: 0}, nil
			}
			// compose up returns non-zero exit.
			return &remote.CommandResult{ExitCode: 1, Stderr: "compose: image not found"}, nil
		},
		copyDirFunc: func(_ context.Context, _ remote.RemoteHost, _, _ string) error {
			return nil
		},
	}

	o := New(WithRemoteExecutor(exec), WithHostManager(hostMgr))
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose up failed")
}

// TestStartRemote_WithProfile exercises the svc.Profile != "" branch which
// builds a different compose command string.
func TestStartRemote_WithProfile(t *testing.T) {
	hostMgr := &moreTestHostMgr{
		hosts: []remote.RemoteHost{{Name: "h1", Address: "10.0.0.1", User: "u"}},
	}
	var capturedCmd string
	callCount := 0
	exec := &moreTestRemoteExec{
		executeFunc: func(_ context.Context, _ remote.RemoteHost, cmd string) (*remote.CommandResult, error) {
			callCount++
			capturedCmd = cmd
			return &remote.CommandResult{ExitCode: 0}, nil
		},
		copyDirFunc: func(_ context.Context, _ remote.RemoteHost, _, _ string) error {
			return nil
		},
	}

	o := New(WithRemoteExecutor(exec), WithHostManager(hostMgr))
	o.remoteEnabled = true

	svc := Service{Name: "test", ComposeFile: "docker-compose.yml", Profile: "core"}
	err := o.startRemote(context.Background(), svc, "/tmp/docker-compose.yml")
	assert.NoError(t, err)
	assert.Contains(t, capturedCmd, "--profile")
	assert.Contains(t, capturedCmd, "core")
}

// --- StopAll additional branch coverage ---

// TestStopAll_NoOrchestrator verifies that StopAll returns nil when
// localOrch is nil (the continue branch).
func TestStopAll_NoOrchestrator(t *testing.T) {
	o := New()
	o.AddService(Service{Name: "svc1", ComposeFile: "docker-compose.yml"})
	o.AddService(Service{Name: "svc2", ComposeFile: "docker-compose.yml"})
	// No local orchestrator set — should continue for every service.
	err := o.StopAll(context.Background())
	assert.NoError(t, err)
}

// TestStopAll_WithError exercises the branch where Down returns an error:
// the error is stored and returned at the end.
func TestStopAll_WithError(t *testing.T) {
	mockOrch := &moreComposeOrch{
		downErr: fmt.Errorf("compose down: network timeout"),
	}
	o := New(WithLocalOrchestrator(mockOrch))
	o.AddService(Service{Name: "svc1", ComposeFile: "docker-compose.yml"})
	err := o.StopAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose down: network timeout")
}

// TestStopAll_Multiple exercises StopAll with multiple services, all
// succeeding.
func TestStopAll_Multiple(t *testing.T) {
	mockOrch := &moreComposeOrch{}
	o := New(WithLocalOrchestrator(mockOrch))
	o.AddService(Service{Name: "svc1", ComposeFile: "dc1.yml"})
	o.AddService(Service{Name: "svc2", ComposeFile: "dc2.yml"})
	o.AddService(Service{Name: "svc3", ComposeFile: "dc3.yml"})
	err := o.StopAll(context.Background())
	assert.NoError(t, err)
	assert.True(t, mockOrch.downCalled)
}

// TestStopAll_RelativePath exercises the relative-path join in StopAll.
func TestStopAll_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	mockOrch := &moreComposeOrch{}
	o := New(WithLocalOrchestrator(mockOrch), WithProjectDir(tmpDir))
	o.AddService(Service{Name: "svc", ComposeFile: "relative/docker-compose.yml"})
	err := o.StopAll(context.Background())
	assert.NoError(t, err)
}
