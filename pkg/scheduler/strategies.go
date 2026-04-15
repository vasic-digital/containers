package scheduler

import (
	"fmt"
	"sort"
	"sync/atomic"

	"digital.vasic.containers/pkg/remote"
)

type hostScore struct {
	name  string
	score float64
}

// scheduleResourceAware places the container on the host with the
// highest resource score.
func scheduleResourceAware(
	scorer *ResourceScorer,
	snapshots map[string]*remote.HostResources,
	hosts []remote.RemoteHost,
	req ContainerRequirements,
	localName string,
) PlacementDecision {

	var candidates []hostScore

	// Score local host if snapshot exists.
	if snap, ok := snapshots[localName]; ok {

		if scorer.CanFit(snap, req) {
			score := scorer.Score(snap, req)

			candidates = append(candidates, hostScore{
				name:  localName,
				score: score,
			})
		} else {

		}
	}

	// Score remote hosts.
	for _, h := range hosts {
		snap, ok := snapshots[h.Name]
		if !ok {
			continue
		}

		if !labelsMatch(h.Labels, req.Labels) {

			continue
		}
		if !scorer.CanFit(snap, req) {

			continue
		}
		score := scorer.Score(snap, req)

		candidates = append(candidates, hostScore{
			name:  h.Name,
			score: score,
		})
	}

	if len(candidates) == 0 {

		return PlacementDecision{
			Requirement: req,
			Score:       0,
			Reason:      "no host has sufficient resources",
		}
	}

	// Prefer local if requested and available.
	if req.PreferLocal {
		for _, c := range candidates {
			if c.name == localName {
				return PlacementDecision{
					Requirement: req,
					HostName:    c.name,
					Score:       c.score,
					Reason:      "preferred local placement",
				}
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	best := candidates[0]

	return PlacementDecision{
		Requirement: req,
		HostName:    best.name,
		Score:       best.score,
		Reason: fmt.Sprintf(
			"best resource score %.3f", best.score,
		),
	}
}

// roundRobinCounter is a package-level counter for round-robin.
var roundRobinCounter atomic.Uint64

// scheduleRoundRobin distributes containers evenly across hosts.
func scheduleRoundRobin(
	hosts []remote.RemoteHost,
	req ContainerRequirements,
	localName string,
) PlacementDecision {
	allNames := []string{localName}
	for _, h := range hosts {
		if labelsMatch(h.Labels, req.Labels) {
			allNames = append(allNames, h.Name)
		}
	}

	if len(allNames) == 0 {
		return PlacementDecision{
			Requirement: req,
			Score:       0,
			Reason:      "no eligible hosts",
		}
	}

	idx := roundRobinCounter.Add(1) - 1
	selected := allNames[idx%uint64(len(allNames))]

	return PlacementDecision{
		Requirement: req,
		HostName:    selected,
		Score:       1.0 / float64(len(allNames)),
		Reason: fmt.Sprintf(
			"round-robin index %d", idx,
		),
	}
}

// scheduleAffinity places containers only on hosts with matching
// labels.
func scheduleAffinity(
	scorer *ResourceScorer,
	snapshots map[string]*remote.HostResources,
	hosts []remote.RemoteHost,
	req ContainerRequirements,
) PlacementDecision {
	var best *hostScore
	for _, h := range hosts {
		if !labelsMatch(h.Labels, req.Labels) {
			continue
		}
		snap, ok := snapshots[h.Name]
		if !ok || !scorer.CanFit(snap, req) {
			continue
		}
		s := scorer.Score(snap, req)
		if best == nil || s > best.score {
			best = &hostScore{name: h.Name, score: s}
		}
	}

	if best == nil {
		return PlacementDecision{
			Requirement: req,
			Score:       0,
			Reason:      "no host matches affinity labels",
		}
	}

	return PlacementDecision{
		Requirement: req,
		HostName:    best.name,
		Score:       best.score,
		Reason:      "affinity label match",
	}
}

// scheduleSpread distributes to minimize per-host container count.
func scheduleSpread(
	snapshots map[string]*remote.HostResources,
	hosts []remote.RemoteHost,
	req ContainerRequirements,
	localName string,
	existing map[string]int,
) PlacementDecision {
	allNames := []string{localName}
	for _, h := range hosts {
		if labelsMatch(h.Labels, req.Labels) {
			allNames = append(allNames, h.Name)
		}
	}

	if len(allNames) == 0 {
		return PlacementDecision{
			Requirement: req,
			Score:       0,
			Reason:      "no eligible hosts",
		}
	}

	// Pick host with fewest existing containers.
	sort.Slice(allNames, func(i, j int) bool {
		return existing[allNames[i]] < existing[allNames[j]]
	})

	selected := allNames[0]
	return PlacementDecision{
		Requirement: req,
		HostName:    selected,
		Score:       0.5,
		Reason: fmt.Sprintf(
			"spread: fewest containers (%d)",
			existing[selected],
		),
	}
}

// scheduleBinPack packs containers onto as few hosts as possible.
func scheduleBinPack(
	scorer *ResourceScorer,
	snapshots map[string]*remote.HostResources,
	hosts []remote.RemoteHost,
	req ContainerRequirements,
	localName string,
) PlacementDecision {
	type candidate struct {
		name string
		used float64
	}

	var candidates []candidate

	if snap, ok := snapshots[localName]; ok {
		if scorer.CanFit(snap, req) {
			candidates = append(candidates, candidate{
				name: localName,
				used: snap.CPUPercent + snap.MemoryPercent,
			})
		}
	}

	for _, h := range hosts {
		if !labelsMatch(h.Labels, req.Labels) {
			continue
		}
		snap, ok := snapshots[h.Name]
		if !ok || !scorer.CanFit(snap, req) {
			continue
		}
		candidates = append(candidates, candidate{
			name: h.Name,
			used: snap.CPUPercent + snap.MemoryPercent,
		})
	}

	if len(candidates) == 0 {
		return PlacementDecision{
			Requirement: req,
			Score:       0,
			Reason:      "no host can fit container",
		}
	}

	// Pick the most-used host that can still fit.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].used > candidates[j].used
	})

	selected := candidates[0]
	return PlacementDecision{
		Requirement: req,
		HostName:    selected.name,
		Score:       0.5,
		Reason: fmt.Sprintf(
			"bin-pack: most utilized host (%.1f%%)",
			selected.used/2,
		),
	}
}

// labelsMatch returns true if the host has all required labels.
func labelsMatch(
	hostLabels, requiredLabels map[string]string,
) bool {
	for k, v := range requiredLabels {
		if hostLabels[k] != v {
			return false
		}
	}
	return true
}
