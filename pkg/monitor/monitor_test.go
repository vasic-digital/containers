package monitor_test

import (
	"context"
	"io"
	"testing"
	"time"

	"digital.vasic.containers/pkg/monitor"
	"digital.vasic.containers/pkg/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRuntime is a minimal ContainerRuntime for testing the monitor.
type stubRuntime struct {
	containers []runtime.ContainerInfo
	stats      map[string]*runtime.ContainerStats
}

func (s *stubRuntime) Name() string { return "stub" }
func (s *stubRuntime) Version(_ context.Context) (string, error) {
	return "0.0.0", nil
}
func (s *stubRuntime) IsAvailable(_ context.Context) bool {
	return true
}
func (s *stubRuntime) Start(
	_ context.Context, _ string, _ ...runtime.StartOption,
) error {
	return nil
}
func (s *stubRuntime) Stop(
	_ context.Context, _ string, _ ...runtime.StopOption,
) error {
	return nil
}
func (s *stubRuntime) Remove(
	_ context.Context, _ string, _ ...runtime.RemoveOption,
) error {
	return nil
}
func (s *stubRuntime) Status(
	_ context.Context, _ string,
) (*runtime.ContainerStatus, error) {
	return &runtime.ContainerStatus{}, nil
}
func (s *stubRuntime) List(
	_ context.Context, _ runtime.ListFilter,
) ([]runtime.ContainerInfo, error) {
	return s.containers, nil
}
func (s *stubRuntime) Stats(
	_ context.Context, id string,
) (*runtime.ContainerStats, error) {
	if st, ok := s.stats[id]; ok {
		return st, nil
	}
	return &runtime.ContainerStats{}, nil
}
func (s *stubRuntime) Exec(
	_ context.Context, _ string, _ []string,
) (*runtime.ExecResult, error) {
	return &runtime.ExecResult{}, nil
}
func (s *stubRuntime) Logs(
	_ context.Context, _ string, _ ...runtime.LogOption,
) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}

// stubSystemCollector returns fixed system metrics.
type stubSystemCollector struct {
	cpu    float64
	memory float64
}

func (c *stubSystemCollector) Collect() monitor.SystemResources {
	return monitor.SystemResources{
		CPUPercent:    c.cpu,
		MemoryPercent: c.memory,
	}
}

func TestDefaultMonitor_Snapshot_NoData(t *testing.T) {
	m := monitor.NewDefaultMonitor(nil, nil)
	_, err := m.Snapshot()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no snapshot available")
}

func TestDefaultMonitor_StartStop(t *testing.T) {
	sys := &stubSystemCollector{cpu: 25.0, memory: 50.0}
	rt := &stubRuntime{
		containers: []runtime.ContainerInfo{
			{ID: "c1", Name: "redis"},
		},
		stats: map[string]*runtime.ContainerStats{
			"c1": {CPUPercent: 10, MemoryPercent: 20},
		},
	}
	m := monitor.NewDefaultMonitor(rt, sys)

	ctx, cancel := context.WithTimeout(
		context.Background(), 300*time.Millisecond,
	)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- m.Start(ctx, 100*time.Millisecond)
	}()

	// Wait for at least one snapshot to be collected.
	time.Sleep(150 * time.Millisecond)

	snap, err := m.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, 25.0, snap.System.CPUPercent)
	assert.Equal(t, 50.0, snap.System.MemoryPercent)
	assert.Contains(t, snap.Containers, "redis")
	assert.Equal(t, 10.0, snap.Containers["redis"].CPUPercent)

	// Stop via context cancellation; wait for Start to return.
	cancel()
	<-done
}

func TestDefaultMonitor_InvalidInterval(t *testing.T) {
	m := monitor.NewDefaultMonitor(nil, nil)
	err := m.Start(context.Background(), 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interval must be positive")
}

func TestDefaultMonitor_StopIdempotent(t *testing.T) {
	m := monitor.NewDefaultMonitor(nil, nil)
	assert.NoError(t, m.Stop())
	assert.NoError(t, m.Stop())
}

func TestDefaultMonitor_SetThreshold(t *testing.T) {
	sys := &stubSystemCollector{cpu: 90.0, memory: 50.0}
	m := monitor.NewDefaultMonitor(nil, sys)

	triggered := make(chan struct{}, 1)
	m.SetThreshold(monitor.ThresholdRule{
		Metric:    "system.cpu",
		Threshold: 80.0,
		Operator:  ">",
		Action: func(_ *monitor.ResourceSnapshot) {
			select {
			case triggered <- struct{}{}:
			default:
			}
		},
	})

	ctx, cancel := context.WithTimeout(
		context.Background(), 250*time.Millisecond,
	)
	defer cancel()

	go func() {
		_ = m.Start(ctx, 100*time.Millisecond)
	}()

	select {
	case <-triggered:
		// Success: threshold was triggered.
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected threshold to trigger")
	}
	cancel()
}
