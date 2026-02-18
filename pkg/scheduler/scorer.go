package scheduler

import (
	"digital.vasic.containers/pkg/remote"
)

// ResourceScorer scores a host from 0.0 to 1.0 based on available
// resources and configured weights.
type ResourceScorer struct {
	opts Options
}

// NewResourceScorer creates a scorer with the given options.
func NewResourceScorer(opts Options) *ResourceScorer {
	return &ResourceScorer{opts: opts}
}

// Score evaluates a host's suitability for a container. Returns a
// value between 0.0 (worst) and 1.0 (best).
func (s *ResourceScorer) Score(
	resources *remote.HostResources,
	req ContainerRequirements,
) float64 {
	if resources == nil {
		return 0
	}

	reserve := s.opts.ReservePercent
	overcommit := s.opts.OvercommitRatio
	if overcommit <= 0 {
		overcommit = 1.0
	}

	// CPU score: available CPU relative to requirement.
	cpuScore := s.scoreCPU(resources, req, reserve, overcommit)

	// Memory score: available memory relative to requirement.
	memScore := s.scoreMemory(resources, req, reserve, overcommit)

	// Disk score: available disk relative to requirement.
	diskScore := s.scoreDisk(resources, req, reserve)

	// Network score: inversely proportional to network usage.
	netScore := s.scoreNetwork(resources)

	// Weighted sum.
	total := cpuScore*s.opts.CPUWeight +
		memScore*s.opts.MemoryWeight +
		diskScore*s.opts.DiskWeight +
		netScore*s.opts.NetworkWeight

	return clamp(total, 0, 1)
}

// CanFit returns true if the host has enough resources for the
// container, accounting for reserves.
func (s *ResourceScorer) CanFit(
	resources *remote.HostResources,
	req ContainerRequirements,
) bool {
	if resources == nil {
		return false
	}

	reserve := s.opts.ReservePercent
	overcommit := s.opts.OvercommitRatio
	if overcommit <= 0 {
		overcommit = 1.0
	}

	// Check CPU.
	if req.CPUCores > 0 {
		availCores := float64(resources.CPUCores) *
			(resources.AvailableCPUPercent() - reserve) / 100.0 *
			overcommit
		if availCores < req.CPUCores {
			return false
		}
	}

	// Check memory.
	if req.MemoryMB > 0 {
		availMB := float64(resources.MemoryTotalMB) *
			(resources.AvailableMemoryPercent() - reserve) / 100.0 *
			overcommit
		if availMB < 0 || uint64(availMB) < req.MemoryMB {
			return false
		}
	}

	// Check disk.
	if req.DiskMB > 0 {
		availDiskMB := float64(resources.DiskTotalMB) *
			(resources.AvailableDiskPercent() - reserve) / 100.0
		if availDiskMB < 0 || uint64(availDiskMB) < req.DiskMB {
			return false
		}
	}

	return true
}

func (s *ResourceScorer) scoreCPU(
	r *remote.HostResources,
	req ContainerRequirements,
	reserve, overcommit float64,
) float64 {
	avail := r.AvailableCPUPercent() - reserve
	if avail <= 0 {
		return 0
	}
	score := avail / 100.0
	if req.CPUCores > 0 && r.CPUCores > 0 {
		availCores := float64(r.CPUCores) * avail / 100.0 * overcommit
		if availCores < req.CPUCores {
			return 0
		}
		score = (availCores - req.CPUCores) / float64(r.CPUCores)
	}
	return clamp(score, 0, 1)
}

func (s *ResourceScorer) scoreMemory(
	r *remote.HostResources,
	req ContainerRequirements,
	reserve, overcommit float64,
) float64 {
	avail := r.AvailableMemoryPercent() - reserve
	if avail <= 0 {
		return 0
	}
	score := avail / 100.0
	if req.MemoryMB > 0 && r.MemoryTotalMB > 0 {
		availMB := float64(r.MemoryTotalMB) * avail / 100.0 * overcommit
		if uint64(availMB) < req.MemoryMB {
			return 0
		}
		score = (availMB - float64(req.MemoryMB)) /
			float64(r.MemoryTotalMB)
	}
	return clamp(score, 0, 1)
}

func (s *ResourceScorer) scoreDisk(
	r *remote.HostResources,
	req ContainerRequirements,
	reserve float64,
) float64 {
	avail := r.AvailableDiskPercent() - reserve
	if avail <= 0 {
		return 0
	}
	score := avail / 100.0
	if req.DiskMB > 0 && r.DiskTotalMB > 0 {
		availMB := float64(r.DiskTotalMB) * avail / 100.0
		if uint64(availMB) < req.DiskMB {
			return 0
		}
		score = (availMB - float64(req.DiskMB)) /
			float64(r.DiskTotalMB)
	}
	return clamp(score, 0, 1)
}

func (s *ResourceScorer) scoreNetwork(
	r *remote.HostResources,
) float64 {
	// Simple heuristic: lower network usage is better.
	// Normalize to a reasonable range (1 Gbps = 125MB/s).
	const maxBytesPerSec = 125_000_000
	totalUsage := float64(
		r.NetworkRxBytesPerSec + r.NetworkTxBytesPerSec,
	)
	if totalUsage >= maxBytesPerSec {
		return 0
	}
	return 1.0 - (totalUsage / maxBytesPerSec)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
