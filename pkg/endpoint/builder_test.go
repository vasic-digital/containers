package endpoint

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuilder_Defaults(t *testing.T) {
	ep := NewEndpoint().Build()
	assert.True(t, ep.Enabled)
	assert.Equal(t, "http", ep.HealthType)
	assert.Equal(t, 10*time.Second, ep.Timeout)
	assert.Equal(t, 3, ep.RetryCount)
}

func TestBuilder_AllFields(t *testing.T) {
	ep := NewEndpoint().
		WithHost("db.local").
		WithPort("5432").
		WithURL("http://db.local:5432").
		WithEnabled(false).
		WithRequired(true).
		WithRemote(true).
		WithHealthPath("/health").
		WithHealthType("tcp").
		WithTimeout(30 * time.Second).
		WithRetryCount(5).
		WithComposeFile("docker-compose.yml").
		WithServiceName("postgres").
		WithProfile("core").
		WithDiscoveryEnabled(true).
		WithDiscoveryMethod("dns").
		WithDiscoveryTimeout(5 * time.Second).
		Build()

	assert.Equal(t, "db.local", ep.Host)
	assert.Equal(t, "5432", ep.Port)
	assert.Equal(t, "http://db.local:5432", ep.URL)
	assert.False(t, ep.Enabled)
	assert.True(t, ep.Required)
	assert.True(t, ep.Remote)
	assert.Equal(t, "/health", ep.HealthPath)
	assert.Equal(t, "tcp", ep.HealthType)
	assert.Equal(t, 30*time.Second, ep.Timeout)
	assert.Equal(t, 5, ep.RetryCount)
	assert.Equal(t, "docker-compose.yml", ep.ComposeFile)
	assert.Equal(t, "postgres", ep.ServiceName)
	assert.Equal(t, "core", ep.Profile)
	assert.True(t, ep.DiscoveryEnabled)
	assert.Equal(t, "dns", ep.DiscoveryMethod)
	assert.Equal(t, 5*time.Second, ep.DiscoveryTimeout)
}

func TestBuilder_Chaining(t *testing.T) {
	b := NewEndpoint()
	result := b.WithHost("h").WithPort("p")
	assert.Same(t, b, result, "builder methods must return same pointer")
}

func TestBuilder_ResolvedURL(t *testing.T) {
	ep := NewEndpoint().
		WithHost("myhost").
		WithPort("9090").
		Build()
	assert.Equal(t, "http://myhost:9090", ep.ResolvedURL())
}

func TestBuilder_ResolvedURL_ExplicitOverride(t *testing.T) {
	ep := NewEndpoint().
		WithHost("myhost").
		WithPort("9090").
		WithURL("https://override.com").
		Build()
	assert.Equal(t, "https://override.com", ep.ResolvedURL())
}

// TestResolveURL_Internal tests the internal resolveURL function directly.
func TestResolveURL_Internal(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		path     string
		expected string
	}{
		{
			name:     "host port path",
			host:     "api.local",
			port:     "8080",
			path:     "health",
			expected: "http://api.local:8080/health",
		},
		{
			name:     "path with leading slash",
			host:     "api.local",
			port:     "8080",
			path:     "/health",
			expected: "http://api.local:8080/health",
		},
		{
			name:     "https host with path",
			host:     "https://secure.local",
			port:     "443",
			path:     "api/v1",
			expected: "https://secure.local:443/api/v1",
		},
		{
			name:     "empty path",
			host:     "api.local",
			port:     "8080",
			path:     "",
			expected: "http://api.local:8080",
		},
		{
			name:     "empty host defaults to localhost",
			host:     "",
			port:     "8080",
			path:     "test",
			expected: "http://localhost:8080/test",
		},
		{
			name:     "no port with path",
			host:     "api.local",
			port:     "",
			path:     "/endpoint",
			expected: "http://api.local/endpoint",
		},
		{
			name:     "http prefix already present",
			host:     "http://api.local",
			port:     "8080",
			path:     "status",
			expected: "http://api.local:8080/status",
		},
		{
			name:     "multiple leading slashes in path",
			host:     "api.local",
			port:     "80",
			path:     "///path",
			expected: "http://api.local:80/path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveURL(tc.host, tc.port, tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}
}
