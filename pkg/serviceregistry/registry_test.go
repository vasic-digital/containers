package serviceregistry

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry(t *testing.T) *ServiceRegistry {
	tmpDir, err := os.MkdirTemp("", "service-registry-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	return New(WithRegistryDir(tmpDir))
}

func TestNew(t *testing.T) {
	r := newTestRegistry(t)
	require.NotNil(t, r)
	assert.NotNil(t, r.services)
}

func TestRegister(t *testing.T) {
	r := newTestRegistry(t)
	err := r.Register("test-service", 8080, WithHealthPath("/health"))
	require.NoError(t, err)

	svc, ok := r.Get("test-service")
	require.True(t, ok)
	assert.Equal(t, "test-service", svc.Name)
	assert.Equal(t, 8080, svc.Port)
	assert.Equal(t, "/health", svc.HealthPath)
}

func TestGet_NonExistent(t *testing.T) {
	r := newTestRegistry(t)
	svc, ok := r.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, svc)
}

func TestGetEndpoint(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("postgres", 5432)

	endpoint := r.GetEndpoint("postgres")
	assert.Equal(t, "localhost:5432", endpoint)

	endpoint = r.GetEndpoint("nonexistent")
	assert.Empty(t, endpoint)
}

func TestGetURL(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("api", 8080)

	url := r.GetURL("api")
	assert.Equal(t, "http://localhost:8080", url)
}

func TestGetAll(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("svc1", 8001)
	_ = r.Register("svc2", 8002)

	all := r.GetAll()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "svc1")
	assert.Contains(t, all, "svc2")
}

func TestUnregister(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("temp", 9000)

	_, ok := r.Get("temp")
	assert.True(t, ok)

	r.Unregister("temp")

	_, ok = r.Get("temp")
	assert.False(t, ok)
}

func TestUpdateHealth(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("health-check", 9000, WithHealthPath("/health"))

	svc, _ := r.Get("health-check")
	assert.True(t, svc.Healthy)

	r.UpdateHealth("health-check", false)

	svc, _ = r.Get("health-check")
	assert.False(t, svc.Healthy)
}

func TestDiscover_PortFound(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	r := newTestRegistry(t)
	svc, err := r.Discover(context.Background(), "discovered", port, port, port+10)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, port, svc.Port)
}

func TestDiscover_PortNotFound(t *testing.T) {
	r := newTestRegistry(t)
	svc, err := r.Discover(context.Background(), "notfound", 19999, 19999, 20000)
	assert.Error(t, err)
	assert.Nil(t, svc)
	assert.Contains(t, err.Error(), "not discovered")
}

func TestFindAvailablePort(t *testing.T) {
	r := newTestRegistry(t)
	port := r.FindAvailablePort(20000)
	assert.GreaterOrEqual(t, port, 20000)
	assert.Less(t, port, 30000)
}

func TestWithDefaultHost(t *testing.T) {
	r := newTestRegistry(t)
	r.defaultHost = "192.168.1.1"
	_ = r.Register("test", 8080)

	svc, ok := r.Get("test")
	require.True(t, ok)
	assert.Equal(t, "192.168.1.1", svc.Host)
}

func TestWithLabels(t *testing.T) {
	r := newTestRegistry(t)
	err := r.Register("labeled", 8080, WithLabels(map[string]string{
		"env":     "test",
		"service": "api",
	}))
	require.NoError(t, err)

	svc, ok := r.Get("labeled")
	require.True(t, ok)
	assert.Equal(t, "test", svc.Labels["env"])
	assert.Equal(t, "api", svc.Labels["service"])
}

func TestGlobal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "global-registry-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	SetGlobal(nil)
	r1 := Global()
	require.NotNil(t, r1)

	r2 := Global()
	assert.Equal(t, r1, r2)
	SetGlobal(nil)
}

func TestClear(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("clear-test", 9000)
	assert.Len(t, r.GetAll(), 1)

	r.Clear()
	assert.Len(t, r.GetAll(), 0)
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "service-registry-persist")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	r1 := New(WithRegistryDir(tmpDir))
	_ = r1.Register("persisted", 5432, WithHealthPath("/health"))

	r2 := New(WithRegistryDir(tmpDir))
	svc, ok := r2.Get("persisted")
	require.True(t, ok)
	assert.Equal(t, 5432, svc.Port)
	assert.Equal(t, "/health", svc.HealthPath)
}

func TestDiscoverMultiple(t *testing.T) {
	ln1, err := net.Listen("tcp", "localhost:28001")
	require.NoError(t, err)
	defer ln1.Close()

	ln2, err := net.Listen("tcp", "localhost:28002")
	require.NoError(t, err)
	defer ln2.Close()

	r := newTestRegistry(t)
	services := map[string]int{
		"svc1": 28001,
		"svc2": 28002,
		"svc3": 28999,
	}

	err = r.DiscoverMultiple(context.Background(), services)
	assert.NoError(t, err)

	_, ok1 := r.Get("svc1")
	assert.True(t, ok1)

	_, ok2 := r.Get("svc2")
	assert.True(t, ok2)

	_, ok3 := r.Get("svc3")
	assert.False(t, ok3)
}

func TestDiscoverContextCancellation(t *testing.T) {
	r := newTestRegistry(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc, err := r.Discover(ctx, "cancelled", 18081, 18081, 18090)
	assert.Error(t, err)
	assert.Nil(t, svc)
}

func TestDiscoverWithExistingRegistration(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("existing", 9999)

	svc, err := r.Discover(context.Background(), "existing", 10000, 10000, 10010)
	require.NoError(t, err)
	assert.Equal(t, 9999, svc.Port)
}

func TestList(t *testing.T) {
	r := newTestRegistry(t)
	_ = r.Register("svc1", 8001)
	_ = r.Register("svc2", 8002)

	list := r.List()
	assert.Len(t, list, 2)

	names := make(map[string]bool)
	for _, svc := range list {
		names[svc.Name] = true
	}
	assert.True(t, names["svc1"])
	assert.True(t, names["svc2"])
}

func TestConcurrentAccess(t *testing.T) {
	r := newTestRegistry(t)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			name := string(rune('A' + id))
			_ = r.Register(name, 8000+id)
			_, _ = r.Get(name)
			r.UpdateHealth(name, true)
			<-time.After(10 * time.Millisecond)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Len(t, r.GetAll(), 10)
}
