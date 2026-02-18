package envconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantKey   string
		wantValue string
		wantOK    bool
	}{
		{
			"simple",
			"KEY=value",
			"KEY", "value", true,
		},
		{
			"with spaces",
			"KEY = value",
			"KEY", "value", true,
		},
		{
			"double quoted",
			`KEY="hello world"`,
			"KEY", "hello world", true,
		},
		{
			"single quoted",
			"KEY='hello world'",
			"KEY", "hello world", true,
		},
		{
			"inline comment",
			"KEY=value # this is a comment",
			"KEY", "value", true,
		},
		{
			"export prefix",
			"export KEY=value",
			"KEY", "value", true,
		},
		{
			"empty value",
			"KEY=",
			"KEY", "", true,
		},
		{
			"no equals",
			"INVALID_LINE",
			"", "", false,
		},
		{
			"path value",
			"KEY=/home/user/.ssh/id_rsa",
			"KEY", "/home/user/.ssh/id_rsa", true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, ok := parseDotEnvLine(tt.line)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantKey, key)
				assert.Equal(t, tt.wantValue, value)
			}
		})
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := `# Remote distribution config
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_DEFAULT_SSH_USER=testuser
CONTAINERS_REMOTE_HOST_1_NAME=test-host
CONTAINERS_REMOTE_HOST_1_ADDRESS=10.0.0.1
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	// Clear relevant vars.
	os.Unsetenv("CONTAINERS_REMOTE_ENABLED")
	os.Unsetenv("CONTAINERS_REMOTE_DEFAULT_SSH_USER")
	os.Unsetenv("CONTAINERS_REMOTE_HOST_1_NAME")
	os.Unsetenv("CONTAINERS_REMOTE_HOST_1_ADDRESS")

	err = loadDotEnv(path)
	require.NoError(t, err)

	assert.Equal(t, "true", os.Getenv("CONTAINERS_REMOTE_ENABLED"))
	assert.Equal(t, "testuser",
		os.Getenv("CONTAINERS_REMOTE_DEFAULT_SSH_USER"),
	)
}

func TestLoadDotEnv_EnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := "TEST_DOTENV_PRECEDENCE=from_file\n"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	// Set env before loading.
	t.Setenv("TEST_DOTENV_PRECEDENCE", "from_env")

	err = loadDotEnv(path)
	require.NoError(t, err)

	// Env should take precedence.
	assert.Equal(t, "from_env",
		os.Getenv("TEST_DOTENV_PRECEDENCE"),
	)
}

func TestLoadDotEnv_FileNotFound(t *testing.T) {
	err := loadDotEnv("/nonexistent/.env")
	assert.Error(t, err)
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := `CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_HOST_1_NAME=file-host
CONTAINERS_REMOTE_HOST_1_ADDRESS=192.168.1.50
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	os.Unsetenv("CONTAINERS_REMOTE_ENABLED")
	os.Unsetenv("CONTAINERS_REMOTE_HOST_1_NAME")
	os.Unsetenv("CONTAINERS_REMOTE_HOST_1_ADDRESS")

	cfg, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	require.Len(t, cfg.Hosts, 1)
	assert.Equal(t, "file-host", cfg.Hosts[0].Name)
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/.env")
	assert.Error(t, err)
}

func TestGenerateEnvExample(t *testing.T) {
	template := GenerateEnvExample()
	assert.Contains(t, template, "CONTAINERS_REMOTE_ENABLED")
	assert.Contains(t, template, "CONTAINERS_REMOTE_HOST_1_NAME")
	assert.Contains(t, template, "CONTAINERS_REMOTE_SCHEDULER")
	assert.Contains(t, template, "CONTAINERS_REMOTE_VOLUME_TYPE")
}
