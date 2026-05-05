package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTunnelOverlay_NilLogger exercises nil logger fallback.
// TestTunnelOverlay_Create_And_List exercises Create and List.
func TestTunnelOverlay_Create_And_List(t *testing.T) {
	overlay := NewTunnelOverlay(nil, nil, nil, nil)
	ctx := context.Background()

	require.NoError(t, overlay.Create(ctx, "net-1"))
	require.NoError(t, overlay.Create(ctx, "net-2"))

	// Duplicate create should return an error
	err := overlay.Create(ctx, "net-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	networks, err := overlay.List(ctx)
	require.NoError(t, err)
	assert.Len(t, networks, 2)
}

// TestTunnelOverlay_Delete_Existing exercises a successful Delete.
func TestTunnelOverlay_Delete_Existing(t *testing.T) {
	overlay := NewTunnelOverlay(nil, nil, nil, nil)
	ctx := context.Background()

	require.NoError(t, overlay.Create(ctx, "net-to-delete"))
	require.NoError(t, overlay.Delete(ctx, "net-to-delete"))

	networks, err := overlay.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, networks)
}

// TestTunnelOverlay_Connect_And_Disconnect exercises the full lifecycle.
func TestTunnelOverlay_Connect_And_Disconnect(t *testing.T) {
	overlay := NewTunnelOverlay(nil, nil, nil, nil)
	ctx := context.Background()

	require.NoError(t, overlay.Create(ctx, "app-net"))

	// Connect two containers
	require.NoError(t, overlay.Connect(ctx, "app-net", "container-1"))
	require.NoError(t, overlay.Connect(ctx, "app-net", "container-2"))
	assert.Len(t, overlay.networks["app-net"], 2)

	// Connect to non-existent network
	err := overlay.Connect(ctx, "missing-net", "c3")
	assert.Error(t, err)

	// Disconnect one container
	require.NoError(t, overlay.Disconnect(ctx, "app-net", "container-1"))
	assert.Len(t, overlay.networks["app-net"], 1)
	assert.Equal(t, "container-2", overlay.networks["app-net"][0])

	// Disconnect from non-existent network
	err = overlay.Disconnect(ctx, "missing-net", "c1")
	assert.Error(t, err)
}
