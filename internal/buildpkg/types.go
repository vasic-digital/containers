package buildpkg

import (
	"fmt"
	"time"

	"digital.vasic.containers/pkg/scheduler"
)

type BuildStatus string

const (
	BuildStatusPending BuildStatus = "pending"
	BuildStatusRunning BuildStatus = "running"
	BuildStatusSuccess BuildStatus = "success"
	BuildStatusFailed  BuildStatus = "failed"
	BuildStatusSkipped BuildStatus = "skipped"
)

type BuildComponent struct {
	Name    string
	HasGo   bool
	HasNPM  bool
	HasJDK  bool
	HasRust bool
}

func (c BuildComponent) ResourceRequirements() scheduler.ContainerRequirements {
	var cpu float64
	var mem uint64
	var disk uint64
	labels := make(map[string]string)

	switch {
	case c.HasJDK:
		cpu = 3
		mem = 4096
		disk = 2048
		labels["jdk"] = "true"
	case c.HasRust:
		cpu = 3
		mem = 4096
		disk = 2048
		labels["rust"] = "true"
	case c.HasGo:
		cpu = 2
		mem = 2048
		disk = 1024
		labels["go"] = "true"
	default:
		cpu = 1
		mem = 1024
		disk = 512
	}

	if c.HasNPM {
		labels["npm"] = "true"
	}

	return scheduler.ContainerRequirements{
		Name:     c.Name,
		Image:    "localhost/catalogizer-builder:latest",
		CPUCores: cpu,
		MemoryMB: mem,
		DiskMB:   disk,
		Labels:   labels,
	}
}

type BuildResult struct {
	Component    string
	Host         string
	Status       BuildStatus
	Duration     time.Duration
	ArtifactPath string
	Error        string
}

func (r BuildResult) IsSuccess() bool {
	return r.Status == BuildStatusSuccess
}

func (r BuildResult) IsFailure() bool {
	return r.Status == BuildStatusFailed
}

type BuildAssignment struct {
	Component BuildComponent
	Host      string
}

type BuildPlan struct {
	Assignments []BuildAssignment
}

func (p BuildPlan) LocalAssignments() []BuildAssignment {
	var local []BuildAssignment
	for _, a := range p.Assignments {
		if a.Host == "" || a.Host == "local" {
			local = append(local, a)
		}
	}
	return local
}

func (p BuildPlan) RemoteAssignments() []BuildAssignment {
	var remote []BuildAssignment
	for _, a := range p.Assignments {
		if a.Host != "" && a.Host != "local" {
			remote = append(remote, a)
		}
	}
	return remote
}

func (p BuildPlan) ByHost() map[string][]BuildAssignment {
	groups := make(map[string][]BuildAssignment)
	for _, a := range p.Assignments {
		host := a.Host
		if host == "" {
			host = ""
		}
		groups[host] = append(groups[host], a)
	}
	return groups
}

func AllComponents() []BuildComponent {
	return []BuildComponent{
		{Name: "catalog-api", HasGo: true},
		{Name: "catalog-web", HasNPM: true},
		{Name: "catalogizer-api-client", HasNPM: true},
		{Name: "catalogizer-desktop", HasNPM: true, HasRust: true},
		{Name: "installer-wizard", HasNPM: true, HasRust: true},
		{Name: "catalogizer-android", HasNPM: true, HasJDK: true},
		{Name: "catalogizer-androidtv", HasNPM: true, HasJDK: true},
	}
}

func FindComponent(name string) (BuildComponent, error) {
	for _, c := range AllComponents() {
		if c.Name == name {
			return c, nil
		}
	}
	return BuildComponent{}, fmt.Errorf("component %q not found", name)
}
