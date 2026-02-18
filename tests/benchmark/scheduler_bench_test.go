package benchmark

import (
	"context"
	"testing"
	"time"

	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// BenchmarkResourceScorer_Score measures the performance of
// scoring a host with sample HostResources against a container
// requirement.
func BenchmarkResourceScorer_Score(b *testing.B) {
	opts := scheduler.DefaultSchedulerOptions()
	scorer := scheduler.NewResourceScorer(opts)

	resources := &remote.HostResources{
		Host:                 "bench-host",
		Timestamp:            time.Now(),
		CPUPercent:           35.0,
		MemoryPercent:        50.0,
		MemoryTotalMB:        16384,
		MemoryUsedMB:         8192,
		DiskPercent:          40.0,
		DiskTotalMB:          512000,
		DiskUsedMB:           204800,
		LoadAvg1:             2.5,
		LoadAvg5:             2.0,
		LoadAvg15:            1.8,
		CPUCores:             8,
		RunningContainers:    5,
		NetworkRxBytesPerSec: 1_000_000,
		NetworkTxBytesPerSec: 500_000,
	}

	req := scheduler.ContainerRequirements{
		Name:     "bench-container",
		Image:    "nginx:latest",
		CPUCores: 2.0,
		MemoryMB: 1024,
		DiskMB:   10240,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = scorer.Score(resources, req)
	}
}

// BenchmarkScheduler_ScheduleBatch measures the performance of
// scheduling a batch of 10 container requirements with a host
// manager that has no hosts registered.
func BenchmarkScheduler_ScheduleBatch(b *testing.B) {
	executor, err := remote.NewSSHExecutor(
		logging.NopLogger{},
		remote.WithControlMaster(false),
	)
	if err != nil {
		b.Fatalf("failed to create SSH executor: %v", err)
	}
	defer executor.Close()

	hm := remote.NewHostManager(executor, logging.NopLogger{})
	sched := scheduler.NewScheduler(
		hm, logging.NopLogger{},
	)

	reqs := make(
		[]scheduler.ContainerRequirements, 10,
	)
	for i := range reqs {
		reqs[i] = scheduler.ContainerRequirements{
			Name:     "svc-" + string(rune('a'+i)),
			Image:    "alpine:latest",
			CPUCores: 0.5,
			MemoryMB: 256,
			DiskMB:   1024,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = sched.ScheduleBatch(ctx, reqs)
	}
}
