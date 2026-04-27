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

// DefaultBuilderImage is the fallback builder image reference applied
// when BuildComponent.BuilderImage is empty. It is intentionally a
// generic vasic-owned label so the library ships no project-specific
// default — integrators set their own image (for example
// "localhost/myproject-builder:latest") either on each BuildComponent
// or via the package-level DefaultBuilderImage override at process
// startup.
var DefaultBuilderImage = "localhost/vasic-builder:latest"

type BuildComponent struct {
	Name         string
	HasGo        bool
	HasNPM       bool
	HasJDK       bool
	HasRust      bool
	// BuilderImage pins the container image to schedule this
	// component's build into. When empty the scheduler falls back to
	// DefaultBuilderImage.
	BuilderImage string
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

	image := c.BuilderImage
	if image == "" {
		image = DefaultBuilderImage
	}

	return scheduler.ContainerRequirements{
		Name:     c.Name,
		Image:    image,
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
		groups[a.Host] = append(groups[a.Host], a)
	}
	return groups
}

// FindComponentIn returns the BuildComponent named `name` from the
// caller-supplied slice. The library carries no hardcoded component
// list — callers register their own set via Planner constructors so
// this package remains project-agnostic.
func FindComponentIn(components []BuildComponent, name string) (BuildComponent, error) {
	for _, c := range components {
		if c.Name == name {
			return c, nil
		}
	}
	return BuildComponent{}, fmt.Errorf("component %q not found", name)
}
