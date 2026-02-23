package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOrchestrator_DiscoverServices_NotFound(t *testing.T) {
	o := New()
	err := o.DiscoverServices("/nonexistent/docker/dir")
	assert.Error(t, err)
}

func TestDefaultOrchestrator_DiscoverServices_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	o := New(WithProjectDir(tmpDir))
	err := o.DiscoverServices(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, o.services)
}

func TestDefaultOrchestrator_DiscoverServices_WithComposeFile(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "postgres")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	composeFile := filepath.Join(subDir, "docker-compose.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir))
	err := o.DiscoverServices(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, o.services, 1)
	assert.Equal(t, "postgres", o.services[0].Name)
}

func TestDefaultOrchestrator_DiscoverServices_WithExcludePattern(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "test-svc")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	composeFile := filepath.Join(subDir, "docker-compose.test.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir), WithExcludePattern("*test*"))
	err := o.DiscoverServices(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, o.services)
}

func TestDefaultOrchestrator_AddService(t *testing.T) {
	o := New()
	svc := Service{Name: "redis", ComposeFile: "docker/redis/docker-compose.yml"}
	o.AddService(svc)
	services := o.ListServices()
	assert.Len(t, services, 1)
	assert.Equal(t, "redis", services[0].Name)
}

func TestDefaultOrchestrator_ListServices(t *testing.T) {
	o := New()
	o.AddService(Service{Name: "svc1"})
	o.AddService(Service{Name: "svc2"})
	services := o.ListServices()
	assert.Len(t, services, 2)
}

func TestDefaultOrchestrator_StartAll_NonexistentFile_Skipped(t *testing.T) {
	// When compose file doesn't exist, StartAll skips the service (no error).
	o := New()
	o.AddService(Service{
		Name:        "test-svc",
		ComposeFile: "/nonexistent/docker-compose.yml",
	})
	err := o.StartAll(context.Background())
	// File doesn't exist -> service is skipped, no error
	assert.NoError(t, err)
}

func TestDefaultOrchestrator_StartAll_NoLocalOrch_Required(t *testing.T) {
	// When localOrch is nil and file exists, startLocal fails.
	// Only pushes to errChan if Required=true.
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	require.NoError(t, os.WriteFile(composeFile, []byte("version: '3'\n"), 0644))

	o := New(WithProjectDir(tmpDir))
	o.AddService(Service{
		Name:        "test-svc",
		ComposeFile: composeFile,
		Required:    true,
	})
	err := o.StartAll(context.Background())
	// localOrch is nil -> startLocal returns error -> Required=true -> errChan gets error
	assert.Error(t, err)
}

func TestDefaultOrchestrator_StartService_NotFound(t *testing.T) {
	o := New()
	err := o.StartService(context.Background(), "nonexistent-service")
	assert.Error(t, err)
}

func TestDefaultOrchestrator_StopAll_NoServices(t *testing.T) {
	o := New()
	err := o.StopAll(context.Background())
	// Should not error with no services
	assert.NoError(t, err)
}

func TestDefaultOrchestrator_StopAll_NilLocalOrch_Skips(t *testing.T) {
	// When localOrch is nil, StopAll skips all services and returns nil.
	o := New()
	o.AddService(Service{Name: "test-svc", ComposeFile: "docker-compose.yml"})
	err := o.StopAll(context.Background())
	assert.NoError(t, err)
}

func TestDefaultOrchestrator_StopAll_WithLocalOrch(t *testing.T) {
	mockOrch := &mockComposeOrchestrator{}
	o := New(WithLocalOrchestrator(mockOrch))
	o.AddService(Service{Name: "test-svc", ComposeFile: "docker-compose.yml"})
	err := o.StopAll(context.Background())
	assert.NoError(t, err)
	assert.True(t, mockOrch.downCalled)
}
