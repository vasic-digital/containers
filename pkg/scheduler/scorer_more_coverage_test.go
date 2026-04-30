package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"digital.vasic.containers/pkg/remote"
)

// TestScoreCPU_AllBranches covers all branches in scoreCPU.
func TestScoreCPU_AllBranches(t *testing.T) {
	o := DefaultSchedulerOptions()
	s := NewResourceScorer(o)

	// Branch: avail <= 0 (all CPU used)
	r1 := &remote.HostResources{CPUPercent: 100, CPUCores: 4}
	score1 := s.scoreCPU(r1, ContainerRequirements{}, 0, 1.0)
	assert.Equal(t, 0.0, score1)

	// Branch: no specific request, just available ratio
	r2 := &remote.HostResources{CPUPercent: 20, CPUCores: 4}
	score2 := s.scoreCPU(r2, ContainerRequirements{}, 0, 1.0)
	assert.Greater(t, score2, 0.0)

	// Branch: CPUCores requested, available not enough -> score 0
	// cpuPercent=80 -> avail=20%, cores=4 -> availCores=4*0.2*1.0=0.8, req=2 -> 0.8<2 -> score=0
	r3 := &remote.HostResources{CPUPercent: 80, CPUCores: 4}
	score3 := s.scoreCPU(r3, ContainerRequirements{CPUCores: 2}, 0, 1.0)
	assert.Equal(t, 0.0, score3)

	// Branch: CPUCores requested, fits -> positive score
	// cpuPercent=10 -> avail=90%, cores=16 -> availCores=16*0.9*1.0=14.4, req=2 -> fits
	r4 := &remote.HostResources{CPUPercent: 10, CPUCores: 16}
	score4 := s.scoreCPU(r4, ContainerRequirements{CPUCores: 2}, 0, 1.0)
	assert.Greater(t, score4, 0.0)

	// Branch: CPUCores=0 in resources (prevents division by zero)
	r5 := &remote.HostResources{CPUPercent: 10, CPUCores: 0}
	score5 := s.scoreCPU(r5, ContainerRequirements{CPUCores: 2}, 0, 1.0)
	assert.GreaterOrEqual(t, score5, 0.0)
}

// TestScoreDisk_AllBranches covers all branches in scoreDisk.
func TestScoreDisk_AllBranches(t *testing.T) {
	o := DefaultSchedulerOptions()
	s := NewResourceScorer(o)

	// Branch: avail <= 0
	r1 := &remote.HostResources{DiskPercent: 100, DiskTotalMB: 1000}
	score1 := s.scoreDisk(r1, ContainerRequirements{}, 0)
	assert.Equal(t, 0.0, score1)

	// Branch: no specific request
	r2 := &remote.HostResources{DiskPercent: 30, DiskTotalMB: 500000}
	score2 := s.scoreDisk(r2, ContainerRequirements{}, 0)
	assert.Greater(t, score2, 0.0)

	// Branch: DiskMB requested, not enough -> score 0
	// diskPercent=90 -> avail=10%, total=100MB -> availMB=10MB, req=50MB -> score=0
	r3 := &remote.HostResources{DiskPercent: 90, DiskTotalMB: 100}
	score3 := s.scoreDisk(r3, ContainerRequirements{DiskMB: 50}, 0)
	assert.Equal(t, 0.0, score3)

	// Branch: DiskMB requested, fits
	r4 := &remote.HostResources{DiskPercent: 20, DiskTotalMB: 100000}
	score4 := s.scoreDisk(r4, ContainerRequirements{DiskMB: 100}, 0)
	assert.Greater(t, score4, 0.0)

	// Branch: DiskTotalMB=0 with disk request (no division, skips scoring)
	r5 := &remote.HostResources{DiskPercent: 20, DiskTotalMB: 0}
	score5 := s.scoreDisk(r5, ContainerRequirements{DiskMB: 100}, 0)
	assert.GreaterOrEqual(t, score5, 0.0)
}

// TestScoreNetwork_AllBranches covers scoreNetwork boundaries.
func TestScoreNetwork_AllBranches(t *testing.T) {
	o := DefaultSchedulerOptions()
	s := NewResourceScorer(o)

	// At/above max network usage -> score 0
	r1 := &remote.HostResources{
		NetworkRxBytesPerSec: 62_500_000,
		NetworkTxBytesPerSec: 62_500_000, // sum = 125MB/s = maxBytesPerSec
	}
	score1 := s.scoreNetwork(r1)
	assert.Equal(t, 0.0, score1)

	// Zero network usage -> score 1
	r2 := &remote.HostResources{}
	score2 := s.scoreNetwork(r2)
	assert.Equal(t, 1.0, score2)

	// Partial usage ~10% of max
	r3 := &remote.HostResources{NetworkRxBytesPerSec: 12_500_000}
	score3 := s.scoreNetwork(r3)
	assert.InDelta(t, 0.9, score3, 0.01)
}

// TestScheduleOne_AllStrategies exercises scheduleOne for all strategies.
func TestScheduleOne_AllStrategies(t *testing.T) {
	// bluff-scan: no-assert-ok (enumeration smoke — every strategy/provider/adapter must not panic)
	mgr := newMockHostManager()
	_ = mgr.AddHost(remote.RemoteHost{Name: "host-1", Address: "1.1.1.1", User: "u", Labels: map[string]string{}})
	mgr.snapshots["host-1"] = makeSnapshot("host-1", 20, 30, 16384, 500000, 8)
	mgr.snapshots["local"] = makeSnapshot("local", 50, 60, 8192, 100000, 4)

	req := ContainerRequirements{Name: "test-container", CPUCores: 1, MemoryMB: 512}

	strategies := []PlacementStrategy{
		StrategyRoundRobin,
		StrategyAffinity,
		StrategySpread,
		StrategyBinPack,
		StrategyResourceAware,
		PlacementStrategy("unknown"), // default case
	}

	for _, strat := range strategies {
		sched := NewScheduler(mgr, nil, WithStrategy(strat))
		decision := sched.scheduleOne(mgr.snapshots, mgr.ListHosts(), req)
		_ = decision // just verify it doesn't panic
	}
}

// TestNewScheduler_NilLogger exercises the nil logger branch.
func TestNewScheduler_NilLogger(t *testing.T) {
	mgr := newMockHostManager()
	sched := NewScheduler(mgr, nil)
	assert.NotNil(t, sched)
	// Should not panic when scheduling (uses NopLogger internally)
	req := ContainerRequirements{Name: "test"}
	_, err := sched.Schedule(context.Background(), req)
	assert.NoError(t, err)
}
