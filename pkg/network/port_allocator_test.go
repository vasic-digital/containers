package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortAllocator_Allocate(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)

	port, err := alloc.Allocate("test-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, port, 40000)
	assert.Less(t, port, 40010)
	assert.True(t, alloc.IsAllocated(port))
	assert.Equal(t, 1, alloc.AllocatedCount())
}

func TestPortAllocator_Allocate_Multiple(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)

	ports := make(map[int]bool)
	for i := 0; i < 5; i++ {
		port, err := alloc.Allocate("test")
		require.NoError(t, err)
		assert.False(t, ports[port], "duplicate port %d", port)
		ports[port] = true
	}
	assert.Equal(t, 5, alloc.AllocatedCount())
}

func TestPortAllocator_Release(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)

	port, err := alloc.Allocate("test")
	require.NoError(t, err)
	assert.True(t, alloc.IsAllocated(port))

	alloc.Release(port)
	assert.False(t, alloc.IsAllocated(port))
	assert.Equal(t, 0, alloc.AllocatedCount())
}

func TestPortAllocator_ReleaseAll(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)

	for i := 0; i < 5; i++ {
		_, err := alloc.Allocate("test")
		require.NoError(t, err)
	}
	assert.Equal(t, 5, alloc.AllocatedCount())

	alloc.ReleaseAll()
	assert.Equal(t, 0, alloc.AllocatedCount())
}

func TestPortAllocator_ListAllocations(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)

	_, _ = alloc.Allocate("first")
	_, _ = alloc.Allocate("second")

	allocs := alloc.ListAllocations()
	assert.Len(t, allocs, 2)
}

func TestPortAllocator_IsAllocated_NotAllocated(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)
	assert.False(t, alloc.IsAllocated(40000))
}

func TestPortAllocator_Release_NotAllocated(t *testing.T) {
	alloc := NewPortAllocator(40000, 40010)
	// Should not panic.
	alloc.Release(40000)
	assert.Equal(t, 0, alloc.AllocatedCount())
}

func TestIsPortAvailable(t *testing.T) {
	// Port 0 won't be listened on; large port should be available.
	// This is environment-dependent, so just verify the function
	// doesn't panic.
	_ = isPortAvailable(40099)
}
