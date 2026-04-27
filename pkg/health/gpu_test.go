package health

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type gpuProbe struct {
	freeMB int
	err    error
}

func (p *gpuProbe) Probe(context.Context) (int, error) {
	return p.freeMB, p.err
}

func TestGPUHealthCheck_SufficientVRAM(t *testing.T) {
	c := NewGPUHealthCheck(&gpuProbe{freeMB: 4000}, 2048)
	err := c.Check(context.Background())
	require.NoError(t, err)
}

func TestGPUHealthCheck_InsufficientVRAM(t *testing.T) {
	c := NewGPUHealthCheck(&gpuProbe{freeMB: 1000}, 2048)
	err := c.Check(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrGPUUnhealthy))
}

func TestGPUHealthCheck_ProbeError(t *testing.T) {
	c := NewGPUHealthCheck(&gpuProbe{err: errors.New("boom")}, 2048)
	err := c.Check(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrGPUUnhealthy))
	// Ensure the upstream probe error is chained via %w and still reachable.
	probeErr := errors.New("boom")
	c2 := NewGPUHealthCheck(&gpuProbe{err: probeErr}, 2048)
	err2 := c2.Check(context.Background())
	require.True(t, errors.Is(err2, probeErr))
}
