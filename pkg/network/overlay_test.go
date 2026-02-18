package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
)

func newTestOverlay() *TunnelOverlay {
	return NewTunnelOverlay(nil, nil, nil, logging.NopLogger{})
}

func TestTunnelOverlay_Create(t *testing.T) {
	o := newTestOverlay()

	err := o.Create(context.Background(), "test-net")
	require.NoError(t, err)

	networks, err := o.List(context.Background())
	require.NoError(t, err)
	assert.Contains(t, networks, "test-net")
}

func TestTunnelOverlay_Create_Duplicate(t *testing.T) {
	o := newTestOverlay()

	err := o.Create(context.Background(), "test-net")
	require.NoError(t, err)

	err = o.Create(context.Background(), "test-net")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTunnelOverlay_Delete(t *testing.T) {
	o := newTestOverlay()

	err := o.Create(context.Background(), "test-net")
	require.NoError(t, err)

	err = o.Delete(context.Background(), "test-net")
	require.NoError(t, err)

	networks, _ := o.List(context.Background())
	assert.NotContains(t, networks, "test-net")
}

func TestTunnelOverlay_Delete_NotFound(t *testing.T) {
	o := newTestOverlay()

	err := o.Delete(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTunnelOverlay_Connect(t *testing.T) {
	o := newTestOverlay()

	err := o.Create(context.Background(), "test-net")
	require.NoError(t, err)

	err = o.Connect(
		context.Background(), "test-net", "container-1",
	)
	assert.NoError(t, err)
}

func TestTunnelOverlay_Connect_NoNetwork(t *testing.T) {
	o := newTestOverlay()

	err := o.Connect(
		context.Background(), "nonexistent", "container-1",
	)
	assert.Error(t, err)
}

func TestTunnelOverlay_Disconnect(t *testing.T) {
	o := newTestOverlay()

	_ = o.Create(context.Background(), "test-net")
	_ = o.Connect(
		context.Background(), "test-net", "container-1",
	)
	_ = o.Connect(
		context.Background(), "test-net", "container-2",
	)

	err := o.Disconnect(
		context.Background(), "test-net", "container-1",
	)
	assert.NoError(t, err)
}

func TestTunnelOverlay_Disconnect_NoNetwork(t *testing.T) {
	o := newTestOverlay()

	err := o.Disconnect(
		context.Background(), "nonexistent", "container-1",
	)
	assert.Error(t, err)
}

func TestTunnelOverlay_List_Empty(t *testing.T) {
	o := newTestOverlay()

	networks, err := o.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, networks)
}

func TestTunnelOverlay_Interface(t *testing.T) {
	var _ OverlayNetwork = (*TunnelOverlay)(nil)
}
