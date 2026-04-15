package buildpkg

import (
	"context"
	"testing"
	"time"

	"digital.vasic.containers/pkg/remote"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type execRecord struct {
	host    remote.RemoteHost
	command string
}

type copyRecord struct {
	host      remote.RemoteHost
	localDir  string
	remoteDir string
}

type mockExecutor struct {
	executedCommands []execRecord
	copiedFiles      []copyRecord
	reachable        map[string]bool
	executeResult    *remote.CommandResult
	executeError     error
	executeDelay     time.Duration
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		reachable: make(map[string]bool),
		executeResult: &remote.CommandResult{
			Stdout:   "build completed",
			ExitCode: 0,
			Duration: 5 * time.Second,
		},
	}
}

func (m *mockExecutor) Execute(ctx context.Context, host remote.RemoteHost, command string) (*remote.CommandResult, error) {
	m.executedCommands = append(m.executedCommands, execRecord{host: host, command: command})
	if m.executeDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.executeDelay):
		}
	}
	return m.executeResult, m.executeError
}

func (m *mockExecutor) CopyDir(_ context.Context, host remote.RemoteHost, localDir, remoteDir string) error {
	m.copiedFiles = append(m.copiedFiles, copyRecord{host: host, localDir: localDir, remoteDir: remoteDir})
	return nil
}

func (m *mockExecutor) IsReachable(_ context.Context, host remote.RemoteHost) bool {
	return m.reachable[host.Name]
}

func TestBuildExecutor_SyncSource(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = true

	exec := NewBuildExecutor(mock, "/home/user/Catalogizer", "/project")

	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}
	err := exec.SyncSource(context.Background(), host)
	require.NoError(t, err)

	require.Len(t, mock.copiedFiles, 1)
	assert.Equal(t, "/home/user/Catalogizer", mock.copiedFiles[0].localDir)
	assert.Equal(t, "/project", mock.copiedFiles[0].remoteDir)
	assert.Equal(t, "thinker", mock.copiedFiles[0].host.Name)

	require.Len(t, mock.executedCommands, 1)
	assert.Contains(t, mock.executedCommands[0].command, "mkdir -p /project")
}

func TestBuildExecutor_SyncSourceUnreachable(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = false

	exec := NewBuildExecutor(mock, "/home/user/Catalogizer", "/project")

	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}
	err := exec.SyncSource(context.Background(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not reachable")
	assert.Contains(t, err.Error(), "thinker")
	assert.Contains(t, err.Error(), "thinker.local")
}

func TestBuildExecutor_LaunchRemoteBuild(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = true

	exec := NewBuildExecutor(mock, "/home/user/Catalogizer", "/project")

	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}
	result, err := exec.LaunchRemoteBuild(context.Background(), host, "catalog-api", "2.2.0", true)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "catalog-api", result.Component)
	assert.Equal(t, "thinker", result.Host)
	assert.Equal(t, BuildStatusSuccess, result.Status)
	assert.Greater(t, result.Duration, time.Duration(0))

	require.Len(t, mock.executedCommands, 1)
	cmd := mock.executedCommands[0].command
	assert.Contains(t, cmd, "catalog-api")
	assert.Contains(t, cmd, "--skip-tests")
	assert.Contains(t, cmd, "--force")
	assert.Contains(t, cmd, "--component")
}

func TestBuildExecutor_BuildTimeout(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = true
	mock.executeDelay = 10 * time.Second

	exec := NewBuildExecutor(mock, "/home/user/Catalogizer", "/project").WithBuildTimeout(1 * time.Nanosecond)

	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}
	_, err := exec.LaunchRemoteBuild(context.Background(), host, "catalog-api", "2.2.0", false)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
