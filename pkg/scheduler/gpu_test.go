package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/remote"
)

func TestGPURequirement_ZeroValue(t *testing.T) {
	var g GPURequirement
	require.Zero(t, g.Count)
	require.Zero(t, g.MinVRAMMB)
	require.Empty(t, g.Capabilities)
}

func TestContainerRequirements_GPUOptional(t *testing.T) {
	var req ContainerRequirements
	require.Nil(t, req.GPU)

	req.GPU = &GPURequirement{Count: 1, MinVRAMMB: 2048}
	require.Equal(t, 1, req.GPU.Count)
}

func TestStrategyGPUAffinity_String(t *testing.T) {
	require.Equal(t, PlacementStrategy("gpu_affinity"), StrategyGPUAffinity)
}

func TestScorer_CanFit_GPU_HostHasNoGPU(t *testing.T) {
	s := NewResourceScorer(Options{ReservePercent: 0, OvercommitRatio: 1})
	res := &remote.HostResources{
		CPUCores:      8,
		MemoryTotalMB: 16_000,
	}
	req := ContainerRequirements{
		CPUCores: 1,
		GPU:      &GPURequirement{Count: 1, MinVRAMMB: 1024},
	}
	require.False(t, s.CanFit(res, req))
}

func TestScorer_CanFit_GPU_HostHasEnough(t *testing.T) {
	s := NewResourceScorer(Options{ReservePercent: 0, OvercommitRatio: 1})
	res := &remote.HostResources{
		CPUCores:      8,
		MemoryTotalMB: 16_000,
		GPU: []remote.GPUDevice{
			{Vendor: "nvidia", VRAMFreeMB: 5800, CUDASupported: true},
		},
	}
	req := ContainerRequirements{
		CPUCores: 1,
		GPU: &GPURequirement{
			Count: 1, MinVRAMMB: 2048,
			Vendor: "nvidia", Capabilities: []string{"cuda"},
		},
	}
	require.True(t, s.CanFit(res, req))
}

func TestScorer_CanFit_GPU_VendorMismatch(t *testing.T) {
	s := NewResourceScorer(Options{ReservePercent: 0, OvercommitRatio: 1})
	res := &remote.HostResources{
		CPUCores:      8,
		MemoryTotalMB: 16_000,
		GPU:           []remote.GPUDevice{{Vendor: "amd", VRAMFreeMB: 8000}},
	}
	req := ContainerRequirements{
		CPUCores: 1,
		GPU:      &GPURequirement{Count: 1, Vendor: "nvidia"},
	}
	require.False(t, s.CanFit(res, req))
}

func TestScorer_Score_GPU_HigherWhenMoreVRAM(t *testing.T) {
	s := NewResourceScorer(Options{
		ReservePercent:  0,
		OvercommitRatio: 1,
		CPUWeight:       0.1, MemoryWeight: 0.1,
		DiskWeight: 0.1, NetworkWeight: 0.1,
		GPUWeight: 0.6,
	})
	resLow := &remote.HostResources{
		CPUCores: 8, MemoryTotalMB: 16_000,
		GPU: []remote.GPUDevice{{Vendor: "nvidia", VRAMFreeMB: 2500, CUDASupported: true}},
	}
	resHigh := &remote.HostResources{
		CPUCores: 8, MemoryTotalMB: 16_000,
		GPU: []remote.GPUDevice{{Vendor: "nvidia", VRAMFreeMB: 5800, CUDASupported: true}},
	}
	req := ContainerRequirements{
		CPUCores: 1,
		GPU:      &GPURequirement{Count: 1, MinVRAMMB: 1024, Vendor: "nvidia"},
	}
	require.Greater(t, s.Score(resHigh, req), s.Score(resLow, req))
}

func TestStrategies_GPUAffinity_PicksGPUHost(t *testing.T) {
	s := NewResourceScorer(Options{ReservePercent: 0, OvercommitRatio: 1, GPUWeight: 1})
	gpuHost := &remote.HostResources{
		Host: "gpu", CPUCores: 8, MemoryTotalMB: 32_000,
		GPU: []remote.GPUDevice{{Vendor: "nvidia", VRAMFreeMB: 5800, CUDASupported: true}},
	}
	cpuHost := &remote.HostResources{
		Host: "cpu", CPUCores: 16, MemoryTotalMB: 64_000,
	}
	req := ContainerRequirements{
		CPUCores: 1,
		GPU: &GPURequirement{
			Count: 1, MinVRAMMB: 1024, Vendor: "nvidia",
			Capabilities: []string{"cuda"},
		},
	}
	host, reason := selectByStrategy(
		StrategyGPUAffinity,
		map[string]*remote.HostResources{
			"gpu": gpuHost, "cpu": cpuHost,
		},
		req, s,
	)
	require.Equal(t, "gpu", host)
	require.Contains(t, reason, "gpu_affinity")
}
