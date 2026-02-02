package endpoint

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceEndpoint_ResolvedURL_ExplicitURL(t *testing.T) {
	ep := &ServiceEndpoint{
		URL: "https://api.example.com:9090/v1",
	}
	assert.Equal(t, "https://api.example.com:9090/v1", ep.ResolvedURL())
}

func TestServiceEndpoint_ResolvedURL_HostPort(t *testing.T) {
	ep := &ServiceEndpoint{Host: "db.local", Port: "5432"}
	assert.Equal(t, "http://db.local:5432", ep.ResolvedURL())
}

func TestServiceEndpoint_ResolvedURL_HostOnly(t *testing.T) {
	ep := &ServiceEndpoint{Host: "redis.local"}
	assert.Equal(t, "http://redis.local", ep.ResolvedURL())
}

func TestServiceEndpoint_ResolvedURL_Empty(t *testing.T) {
	ep := &ServiceEndpoint{}
	assert.Equal(t, "http://localhost", ep.ResolvedURL())
}

func TestServiceEndpoint_ResolvedURL_IgnoresHealthPath(
	t *testing.T,
) {
	ep := &ServiceEndpoint{
		Host:       "svc.local",
		Port:       "8080",
		HealthPath: "/healthz",
	}
	// ResolvedURL returns the base URL; HealthPath is not
	// appended. Use ResolveHealthURL for the full health URL.
	assert.Equal(
		t,
		"http://svc.local:8080",
		ep.ResolvedURL(),
	)
}

func TestServiceEndpoint_ResolvedURL_HTTPSHost(t *testing.T) {
	ep := &ServiceEndpoint{
		Host: "https://secure.local",
		Port: "443",
	}
	assert.Equal(
		t,
		"https://secure.local:443",
		ep.ResolvedURL(),
	)
}

func TestResolveHealthURL_WithPath(t *testing.T) {
	ep := &ServiceEndpoint{
		Host:       "svc",
		Port:       "80",
		HealthPath: "/health",
	}
	assert.Equal(t, "http://svc:80/health", ResolveHealthURL(ep))
}

func TestResolveHealthURL_NoPath(t *testing.T) {
	ep := &ServiceEndpoint{Host: "svc", Port: "80"}
	assert.Equal(t, "http://svc:80", ResolveHealthURL(ep))
}

func TestResolveHostPort(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{"both", "db", "5432", "db:5432"},
		{"host only", "db", "", "db"},
		{"empty host", "", "5432", "localhost:5432"},
		{"both empty", "", "", "localhost"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ep := &ServiceEndpoint{Host: tc.host, Port: tc.port}
			assert.Equal(t, tc.expected, ResolveHostPort(ep))
		})
	}
}

func TestResolveScheme(t *testing.T) {
	assert.Equal(t, "https", ResolveScheme(
		&ServiceEndpoint{URL: "https://x.com"},
	))
	assert.Equal(t, "http", ResolveScheme(
		&ServiceEndpoint{URL: "http://x.com"},
	))
	assert.Equal(t, "http", ResolveScheme(&ServiceEndpoint{}))
}

func TestIsLocalEndpoint(t *testing.T) {
	tests := []struct {
		name   string
		ep     ServiceEndpoint
		expect bool
	}{
		{
			"empty host is local",
			ServiceEndpoint{},
			true,
		},
		{
			"localhost is local",
			ServiceEndpoint{Host: "localhost"},
			true,
		},
		{
			"127.0.0.1 is local",
			ServiceEndpoint{Host: "127.0.0.1"},
			true,
		},
		{
			"::1 is local",
			ServiceEndpoint{Host: "::1"},
			true,
		},
		{
			"remote flag overrides",
			ServiceEndpoint{Host: "localhost", Remote: true},
			false,
		},
		{
			"external host is not local",
			ServiceEndpoint{Host: "db.prod.internal"},
			false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, IsLocalEndpoint(&tc.ep))
		})
	}
}

func TestLoadConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/endpoints.yaml"
	content := `endpoints:
  postgres:
    host: localhost
    port: "5432"
    required: true
    health_path: /healthz
    timeout_seconds: 5
  redis:
    host: redis.local
    port: "6379"
    enabled: false
`
	require.NoError(t, writeTestFile(path, content))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.Endpoints, 2)

	eps := cfg.ToServiceEndpoints()
	pg := eps["postgres"]
	assert.Equal(t, "localhost", pg.Host)
	assert.Equal(t, "5432", pg.Port)
	assert.True(t, pg.Required)
	assert.Equal(t, "/healthz", pg.HealthPath)
	assert.Equal(t, 5*time.Second, pg.Timeout)
	assert.True(t, pg.Enabled)

	rd := eps["redis"]
	assert.False(t, rd.Enabled)
}

func TestLoadConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/endpoints.json"
	content := `{
  "endpoints": {
    "api": {
      "host": "api.local",
      "port": "8080",
      "health_type": "tcp",
      "retry_count": 5
    }
  }
}`
	require.NoError(t, writeTestFile(path, content))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	eps := cfg.ToServiceEndpoints()
	api := eps["api"]
	assert.Equal(t, "api.local", api.Host)
	assert.Equal(t, "tcp", api.HealthType)
	assert.Equal(t, 5, api.RetryCount)
}

func TestLoadConfig_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/endpoints.toml"
	require.NoError(t, writeTestFile(path, ""))

	_, err := LoadConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported config format")
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
