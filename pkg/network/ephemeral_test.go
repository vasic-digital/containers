package network

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenEphemeral_ReturnsConnectableListener(t *testing.T) {
	ln, port, err := ListenEphemeral("127.0.0.1")
	require.NoError(t, err)
	require.NotNil(t, ln)
	defer ln.Close()

	assert.Greater(t, port, 0)
	assert.LessOrEqual(t, port, 65535)

	// Prove the listener is genuinely bound and connectable right now,
	// not just that a plausible-looking int came back.
	conn, dialErr := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, dialErr, "expected to be able to dial the port ListenEphemeral just bound")
	_ = conn.Close()
}

func TestListenEphemeral_ConsecutiveCallsReturnDifferentPorts(t *testing.T) {
	// Because ListenEphemeral keeps the first listener open (does not
	// close-then-return-int), the OS must hand out a different port to
	// the second call. If this ever returns the same port twice, the
	// implementation has regressed into the check-then-use TOCTOU shape
	// this function exists to avoid.
	ln1, port1, err := ListenEphemeral("127.0.0.1")
	require.NoError(t, err)
	defer ln1.Close()

	ln2, port2, err := ListenEphemeral("127.0.0.1")
	require.NoError(t, err)
	defer ln2.Close()

	assert.NotEqual(t, port1, port2, "two live ephemeral listeners must never share a port")

	// Both must be independently connectable at the same time.
	conn1, err := net.Dial("tcp", ln1.Addr().String())
	require.NoError(t, err)
	_ = conn1.Close()

	conn2, err := net.Dial("tcp", ln2.Addr().String())
	require.NoError(t, err)
	_ = conn2.Close()
}

func TestListenEphemeral_BindsOnlyToRequestedInterface(t *testing.T) {
	ln, port, err := ListenEphemeral("127.0.0.1")
	require.NoError(t, err)
	defer ln.Close()

	host, _, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host)

	// Connecting via the loopback address the listener was bound to
	// must succeed.
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err)
	_ = conn.Close()
}

func TestListenEphemeralPort_BindsWildcard(t *testing.T) {
	ln, port, err := ListenEphemeralPort()
	require.NoError(t, err)
	defer ln.Close()

	assert.Greater(t, port, 0)

	host, _, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	// Wildcard bind reports as "::" or "0.0.0.0" depending on platform/
	// IP stack; either way it must NOT be a specific loopback-only
	// address, and it must be dialable via loopback.
	assert.NotEqual(t, "127.0.0.1", host)

	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err)
	_ = conn.Close()
}
