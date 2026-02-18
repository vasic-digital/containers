package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTunnelSpec_Fields(t *testing.T) {
	spec := TunnelSpec{
		Direction:   TunnelLocal,
		LocalPort:   "8080",
		RemoteHost:  "localhost",
		RemotePort:  "5432",
		Description: "PostgreSQL tunnel",
	}

	assert.Equal(t, TunnelLocal, spec.Direction)
	assert.Equal(t, "8080", spec.LocalPort)
	assert.Equal(t, "5432", spec.RemotePort)
}

func TestTunnelInfo_Fields(t *testing.T) {
	info := TunnelInfo{
		Spec:      TunnelSpec{Direction: TunnelLocal},
		HostName:  "gpu-1",
		State:     TunnelActive,
		CreatedAt: time.Now(),
		PID:       12345,
	}

	assert.Equal(t, "gpu-1", info.HostName)
	assert.Equal(t, TunnelActive, info.State)
	assert.Equal(t, 12345, info.PID)
}

func TestTunnelDirection_Values(t *testing.T) {
	assert.Equal(t, TunnelDirection("local"), TunnelLocal)
	assert.Equal(t, TunnelDirection("remote"), TunnelRemote)
}

func TestTunnelState_Values(t *testing.T) {
	assert.Equal(t, TunnelState("active"), TunnelActive)
	assert.Equal(t, TunnelState("closed"), TunnelClosed)
	assert.Equal(t, TunnelState("failed"), TunnelFailed)
	assert.Equal(t, TunnelState("reconnecting"), TunnelReconnecting)
}

func TestDefaultTunnelManager_ListTunnels_Empty(t *testing.T) {
	mgr := &DefaultTunnelManager{
		tunnels: make(map[string]*tunnelEntry),
	}
	assert.Empty(t, mgr.ListTunnels())
}

func TestDefaultTunnelManager_CloseTunnel_NotFound(t *testing.T) {
	mgr := &DefaultTunnelManager{
		tunnels: make(map[string]*tunnelEntry),
	}
	err := mgr.CloseTunnel("99999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tunnel")
}

func TestDefaultTunnelManager_CloseAll_Empty(t *testing.T) {
	mgr := &DefaultTunnelManager{
		tunnels: make(map[string]*tunnelEntry),
	}
	err := mgr.CloseAll()
	assert.NoError(t, err)
}

func TestDefaultTunnelManager_CloseAllForHost_Empty(t *testing.T) {
	mgr := &DefaultTunnelManager{
		tunnels: make(map[string]*tunnelEntry),
	}
	err := mgr.CloseAllForHost("nonexistent")
	assert.NoError(t, err)
}
