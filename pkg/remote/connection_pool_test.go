package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHostKey(t *testing.T) {
	tests := []struct {
		name     string
		host     RemoteHost
		expected string
	}{
		{
			"standard",
			RemoteHost{
				User: "deploy", Address: "192.168.1.100", Port: 22,
			},
			"deploy@192.168.1.100:22",
		},
		{
			"default port",
			RemoteHost{
				User: "root", Address: "10.0.0.1", Port: 0,
			},
			"root@10.0.0.1:22",
		},
		{
			"custom port",
			RemoteHost{
				User: "admin", Address: "example.com", Port: 2222,
			},
			"admin@example.com:2222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hostKey(tt.host))
		})
	}
}

func TestBoolToYesNo(t *testing.T) {
	assert.Equal(t, "yes", boolToYesNo(true))
	assert.Equal(t, "no", boolToYesNo(false))
}

func TestNewConnectionPool(t *testing.T) {
	opts := DefaultOptions()
	opts.ControlSocketDir = t.TempDir()

	pool, err := NewConnectionPool(opts)
	assert.NoError(t, err)
	assert.NotNil(t, pool)
	assert.Equal(t, 0, pool.ActiveCount())
	assert.NoError(t, pool.Close())
}

func TestConnectionPool_CloseHost_NotActive(t *testing.T) {
	opts := DefaultOptions()
	opts.ControlSocketDir = t.TempDir()

	pool, err := NewConnectionPool(opts)
	assert.NoError(t, err)

	host := RemoteHost{
		Name:    "test",
		Address: "127.0.0.1",
		User:    "user",
	}
	// Should not error when host is not active.
	err = pool.CloseHost(host)
	assert.NoError(t, err)
	assert.NoError(t, pool.Close())
}

func TestConnectionPool_Close_Empty(t *testing.T) {
	opts := DefaultOptions()
	opts.ControlSocketDir = t.TempDir()

	pool, err := NewConnectionPool(opts)
	assert.NoError(t, err)
	assert.NoError(t, pool.Close())
}

func TestControlEntry_Fields(t *testing.T) {
	entry := &controlEntry{
		host: RemoteHost{
			Name:    "test",
			Address: "192.168.1.100",
			User:    "deploy",
		},
		socketPath: "/tmp/ctrl-192.168.1.100-22",
		refs:       2,
		createdAt:  time.Now(),
	}

	assert.Equal(t, "test", entry.host.Name)
	assert.Equal(t, 2, entry.refs)
	assert.NotEmpty(t, entry.socketPath)
}
