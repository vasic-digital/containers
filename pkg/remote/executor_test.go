package remote

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockExecutor is a test double for RemoteExecutor.
type mockExecutor struct {
	executeFunc       func(ctx context.Context, host RemoteHost, cmd string) (*CommandResult, error)
	executeStreamFunc func(ctx context.Context, host RemoteHost, cmd string) (io.ReadCloser, error)
	copyFileFunc      func(ctx context.Context, host RemoteHost, local, remote string) error
	copyDirFunc       func(ctx context.Context, host RemoteHost, local, remote string) error
	isReachableFunc   func(ctx context.Context, host RemoteHost) bool
}

func (m *mockExecutor) Execute(
	ctx context.Context, host RemoteHost, cmd string,
) (*CommandResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, host, cmd)
	}
	return &CommandResult{Stdout: "ok\n", ExitCode: 0}, nil
}

func (m *mockExecutor) ExecuteStream(
	ctx context.Context, host RemoteHost, cmd string,
) (io.ReadCloser, error) {
	if m.executeStreamFunc != nil {
		return m.executeStreamFunc(ctx, host, cmd)
	}
	return io.NopCloser(strings.NewReader("stream data")), nil
}

func (m *mockExecutor) CopyFile(
	ctx context.Context, host RemoteHost, local, remote string,
) error {
	if m.copyFileFunc != nil {
		return m.copyFileFunc(ctx, host, local, remote)
	}
	return nil
}

func (m *mockExecutor) CopyDir(
	ctx context.Context, host RemoteHost, local, remote string,
) error {
	if m.copyDirFunc != nil {
		return m.copyDirFunc(ctx, host, local, remote)
	}
	return nil
}

func (m *mockExecutor) IsReachable(
	ctx context.Context, host RemoteHost,
) bool {
	if m.isReachableFunc != nil {
		return m.isReachableFunc(ctx, host)
	}
	return true
}

func TestRemoteExecutor_Interface(t *testing.T) {
	// Verify mockExecutor satisfies RemoteExecutor.
	var _ RemoteExecutor = (*mockExecutor)(nil)
}

func TestMockExecutor_Execute(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test",
		Address: "127.0.0.1",
		User:    "user",
	}

	result, err := exec.Execute(
		context.Background(), host, "echo hello",
	)
	assert.NoError(t, err)
	assert.Equal(t, "ok\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
}

func TestMockExecutor_ExecuteStream(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test",
		Address: "127.0.0.1",
		User:    "user",
	}

	reader, err := exec.ExecuteStream(
		context.Background(), host, "tail -f /var/log/syslog",
	)
	assert.NoError(t, err)
	defer reader.Close()

	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	assert.Equal(t, "stream data", string(buf[:n]))
}

func TestMockExecutor_CopyFile(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test",
		Address: "127.0.0.1",
		User:    "user",
	}

	err := exec.CopyFile(
		context.Background(), host,
		"/local/file.txt", "/remote/file.txt",
	)
	assert.NoError(t, err)
}

func TestMockExecutor_CopyDir(t *testing.T) {
	exec := &mockExecutor{}
	host := RemoteHost{
		Name:    "test",
		Address: "127.0.0.1",
		User:    "user",
	}

	err := exec.CopyDir(
		context.Background(), host,
		"/local/dir", "/remote/dir",
	)
	assert.NoError(t, err)
}

func TestMockExecutor_IsReachable(t *testing.T) {
	exec := &mockExecutor{
		isReachableFunc: func(
			ctx context.Context, host RemoteHost,
		) bool {
			return host.Address == "127.0.0.1"
		},
	}

	host := RemoteHost{
		Name:    "reachable",
		Address: "127.0.0.1",
		User:    "user",
	}
	assert.True(t, exec.IsReachable(context.Background(), host))

	unreachable := RemoteHost{
		Name:    "unreachable",
		Address: "10.0.0.1",
		User:    "user",
	}
	assert.False(t,
		exec.IsReachable(context.Background(), unreachable),
	)
}
