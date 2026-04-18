package health

import (
	"context"
	"errors"
	"fmt"
)

// ErrGPUUnhealthy is returned when the GPU health check fails.
var ErrGPUUnhealthy = errors.New("gpu unhealthy")

// VRAMProbe reports the currently free VRAM in megabytes.
type VRAMProbe interface {
	Probe(ctx context.Context) (freeMB int, err error)
}

// GPUHealthCheck asserts that the probed GPU has at least
// MinFreeVRAMMB available.
type GPUHealthCheck struct {
	probe VRAMProbe
	minMB int
}

// NewGPUHealthCheck wires a probe and a minimum-free-VRAM floor.
func NewGPUHealthCheck(p VRAMProbe, minFreeVRAMMB int) *GPUHealthCheck {
	return &GPUHealthCheck{probe: p, minMB: minFreeVRAMMB}
}

// Check returns nil when VRAM is above the floor, else
// ErrGPUUnhealthy wrapped with detail.
func (c *GPUHealthCheck) Check(ctx context.Context) error {
	if c.probe == nil {
		return fmt.Errorf("%w: no probe configured", ErrGPUUnhealthy)
	}
	free, err := c.probe.Probe(ctx)
	if err != nil {
		return fmt.Errorf("%w: probe: %v", ErrGPUUnhealthy, err)
	}
	if free < c.minMB {
		return fmt.Errorf("%w: free VRAM %d MB < %d MB",
			ErrGPUUnhealthy, free, c.minMB)
	}
	return nil
}
