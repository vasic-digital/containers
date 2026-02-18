package remote

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
)

func newTestHostManager() (*DefaultHostManager, *mockExecutor) {
	exec := &mockExecutor{}
	return NewHostManager(exec, logging.NopLogger{}), exec
}

func testHost(name string) RemoteHost {
	return RemoteHost{
		Name:    name,
		Address: "192.168.1.100",
		Port:    22,
		User:    "deploy",
		Auth:    AuthSSHKey,
		Runtime: "docker",
		Labels:  map[string]string{"env": "test"},
	}
}

func TestDefaultHostManager_AddHost(t *testing.T) {
	mgr, _ := newTestHostManager()

	err := mgr.AddHost(testHost("host-1"))
	require.NoError(t, err)

	hosts := mgr.ListHosts()
	assert.Len(t, hosts, 1)
	assert.Equal(t, "host-1", hosts[0].Name)
}

func TestDefaultHostManager_AddHost_Duplicate(t *testing.T) {
	mgr, _ := newTestHostManager()

	err := mgr.AddHost(testHost("host-1"))
	require.NoError(t, err)

	err = mgr.AddHost(testHost("host-1"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestDefaultHostManager_AddHost_EmptyName(t *testing.T) {
	mgr, _ := newTestHostManager()

	host := testHost("")
	err := mgr.AddHost(host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestDefaultHostManager_AddHost_EmptyAddress(t *testing.T) {
	mgr, _ := newTestHostManager()

	host := testHost("host-1")
	host.Address = ""
	err := mgr.AddHost(host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "address cannot be empty")
}

func TestDefaultHostManager_AddHost_EmptyUser(t *testing.T) {
	mgr, _ := newTestHostManager()

	host := testHost("host-1")
	host.User = ""
	err := mgr.AddHost(host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user cannot be empty")
}

func TestDefaultHostManager_RemoveHost(t *testing.T) {
	mgr, _ := newTestHostManager()

	err := mgr.AddHost(testHost("host-1"))
	require.NoError(t, err)

	err = mgr.RemoveHost("host-1")
	require.NoError(t, err)

	assert.Empty(t, mgr.ListHosts())
}

func TestDefaultHostManager_RemoveHost_NotFound(t *testing.T) {
	mgr, _ := newTestHostManager()

	err := mgr.RemoveHost("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultHostManager_GetHost(t *testing.T) {
	mgr, _ := newTestHostManager()

	expected := testHost("host-1")
	err := mgr.AddHost(expected)
	require.NoError(t, err)

	host, err := mgr.GetHost("host-1")
	require.NoError(t, err)
	assert.Equal(t, expected.Name, host.Name)
	assert.Equal(t, expected.Address, host.Address)
}

func TestDefaultHostManager_GetHost_NotFound(t *testing.T) {
	mgr, _ := newTestHostManager()

	_, err := mgr.GetHost("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultHostManager_ListHosts(t *testing.T) {
	mgr, _ := newTestHostManager()

	for i := 0; i < 3; i++ {
		host := testHost(fmt.Sprintf("host-%d", i))
		host.Address = fmt.Sprintf("192.168.1.%d", 100+i)
		err := mgr.AddHost(host)
		require.NoError(t, err)
	}

	hosts := mgr.ListHosts()
	assert.Len(t, hosts, 3)
}

func TestDefaultHostManager_HostState_Default(t *testing.T) {
	mgr, _ := newTestHostManager()

	assert.Equal(t, HostUnknown, mgr.HostState("nonexistent"))

	err := mgr.AddHost(testHost("host-1"))
	require.NoError(t, err)
	assert.Equal(t, HostUnknown, mgr.HostState("host-1"))
}

func TestDefaultHostManager_ProbeHost_Unreachable(t *testing.T) {
	mgr, exec := newTestHostManager()
	exec.isReachableFunc = func(
		ctx context.Context, host RemoteHost,
	) bool {
		return false
	}

	err := mgr.AddHost(testHost("host-1"))
	require.NoError(t, err)

	_, err = mgr.ProbeHost(context.Background(), "host-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unreachable")
	assert.Equal(t, HostOffline, mgr.HostState("host-1"))
}

func TestDefaultHostManager_ProbeHost_NotFound(t *testing.T) {
	mgr, _ := newTestHostManager()

	_, err := mgr.ProbeHost(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultHostManager_ProbeHost_Success(t *testing.T) {
	mgr, exec := newTestHostManager()
	exec.isReachableFunc = func(
		ctx context.Context, host RemoteHost,
	) bool {
		return true
	}
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		output := "cpu  10000 500 3000 80000 200 100 50 0 0 0\n" +
			"---SEPARATOR---\n" +
			"MemTotal:       16384000 kB\nMemAvailable:    8192000 kB\n" +
			"---SEPARATOR---\n" +
			"1.50 2.30 3.10 3/256 12345\n" +
			"---SEPARATOR---\n" +
			"100000M   60000M\n" +
			"---SEPARATOR---\n" +
			"8\n" +
			"---SEPARATOR---\n" +
			"eth0: 1000000 1000 0 0 0 0 0 0 500000 800 0 0 0 0 0 0\n"
		return &CommandResult{
			Stdout:   output,
			ExitCode: 0,
		}, nil
	}

	err := mgr.AddHost(testHost("host-1"))
	require.NoError(t, err)

	resources, err := mgr.ProbeHost(
		context.Background(), "host-1",
	)
	require.NoError(t, err)
	assert.Equal(t, "host-1", resources.Host)
	assert.Equal(t, HostOnline, mgr.HostState("host-1"))
}

func TestDefaultHostManager_ProbeAll(t *testing.T) {
	mgr, exec := newTestHostManager()
	exec.isReachableFunc = func(
		ctx context.Context, host RemoteHost,
	) bool {
		return true
	}
	exec.executeFunc = func(
		ctx context.Context, host RemoteHost, cmd string,
	) (*CommandResult, error) {
		output := "cpu  10000 500 3000 80000 200 100 50 0 0 0\n" +
			"---SEPARATOR---\n" +
			"MemTotal:       16384000 kB\nMemAvailable:    8192000 kB\n" +
			"---SEPARATOR---\n" +
			"1.50 2.30 3.10 3/256 12345\n" +
			"---SEPARATOR---\n" +
			"100000M   60000M\n" +
			"---SEPARATOR---\n" +
			"8\n" +
			"---SEPARATOR---\n" +
			"eth0: 1000000 1000 0 0 0 0 0 0 500000 800 0 0 0 0 0 0\n"
		return &CommandResult{
			Stdout:   output,
			ExitCode: 0,
		}, nil
	}

	for i := 0; i < 3; i++ {
		host := testHost(fmt.Sprintf("host-%d", i))
		host.Address = fmt.Sprintf("192.168.1.%d", 100+i)
		err := mgr.AddHost(host)
		require.NoError(t, err)
	}

	results := mgr.ProbeAll(context.Background())
	assert.Len(t, results, 3)
}
