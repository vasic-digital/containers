package buildpkg

import (
	"testing"

	"digital.vasic.containers/pkg/scheduler"

	"github.com/stretchr/testify/assert"
)

func TestBuildComponent_ResourceRequirements(t *testing.T) {
	tests := []struct {
		name          string
		component     BuildComponent
		wantCPU       float64
		wantMemory    uint64
		wantDisk      uint64
		wantLabels    map[string]string
		wantLabelKeys []string
	}{
		{
			name: "Go component",
			component: BuildComponent{
				Name:  "svc-api",
				HasGo: true,
			},
			wantCPU:    2,
			wantMemory: 2048,
			wantDisk:   1024,
			wantLabels: map[string]string{
				"go": "true",
			},
		},
		{
			name: "JDK + NPM component",
			component: BuildComponent{
				Name:   "svc-tv",
				HasNPM: true,
				HasJDK: true,
			},
			wantCPU:    3,
			wantMemory: 4096,
			wantDisk:   2048,
			wantLabels: map[string]string{
				"jdk": "true",
				"npm": "true",
			},
		},
		{
			name: "Rust + NPM component",
			component: BuildComponent{
				Name:    "svc-desktop",
				HasNPM:  true,
				HasRust: true,
			},
			wantCPU:    3,
			wantMemory: 4096,
			wantDisk:   2048,
			wantLabels: map[string]string{
				"rust": "true",
				"npm":  "true",
			},
		},
		{
			name: "NPM-only component",
			component: BuildComponent{
				Name:   "svc-web",
				HasNPM: true,
			},
			wantCPU:    1,
			wantMemory: 1024,
			wantDisk:   512,
			wantLabels: map[string]string{
				"npm": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.component.ResourceRequirements()

			assert.Equal(t, tt.wantCPU, req.CPUCores, "CPUCores mismatch")
			assert.Equal(t, tt.wantMemory, req.MemoryMB, "MemoryMB mismatch")
			assert.Equal(t, tt.wantDisk, req.DiskMB, "DiskMB mismatch")
			assert.Equal(t, tt.wantLabels, req.Labels, "Labels mismatch")
			assert.Equal(t, DefaultBuilderImage, req.Image, "Image mismatch — should fall back to DefaultBuilderImage when BuilderImage is empty")
			assert.Equal(t, tt.component.Name, req.Name, "Name mismatch")
		})
	}
}

func TestBuildResult_Success(t *testing.T) {
	r := BuildResult{
		Component: "svc-api",
		Status:    BuildStatusSuccess,
		Duration:  30,
	}
	assert.True(t, r.IsSuccess())
	assert.False(t, r.IsFailure())
}

func TestBuildResult_Failure(t *testing.T) {
	r := BuildResult{
		Component: "svc-web",
		Status:    BuildStatusFailed,
		Error:     "build error",
	}
	assert.False(t, r.IsSuccess())
	assert.True(t, r.IsFailure())
}

func TestBuildPlan_Assignments(t *testing.T) {
	plan := BuildPlan{
		Assignments: []BuildAssignment{
			{
				Component: BuildComponent{Name: "svc-api"},
				Host:      "",
			},
			{
				Component: BuildComponent{Name: "svc-web"},
				Host:      "build-server-1",
			},
			{
				Component: BuildComponent{Name: "svc-android"},
				Host:      "build-server-2",
			},
		},
	}

	local := plan.LocalAssignments()
	assert.Len(t, local, 1)
	assert.Equal(t, "svc-api", local[0].Component.Name)

	remote := plan.RemoteAssignments()
	assert.Len(t, remote, 2)

	byHost := plan.ByHost()
	assert.Len(t, byHost, 3)
	assert.Contains(t, byHost, "")
	assert.Contains(t, byHost, "build-server-1")
	assert.Contains(t, byHost, "build-server-2")
	assert.Len(t, byHost["build-server-1"], 1)
	assert.Equal(t, "svc-web", byHost["build-server-1"][0].Component.Name)
}

func TestFindComponentIn(t *testing.T) {
	// The library carries no hardcoded component list — callers
	// supply their own catalogue. This test proves FindComponentIn
	// resolves a name inside a caller-supplied slice and reports an
	// error for unknown names.
	catalogue := []BuildComponent{
		{Name: "example-api", HasGo: true},
		{Name: "example-web", HasNPM: true},
	}
	api, err := FindComponentIn(catalogue, "example-api")
	assert.NoError(t, err)
	assert.Equal(t, "example-api", api.Name)
	assert.True(t, api.HasGo)

	_, err = FindComponentIn(catalogue, "nonexistent")
	assert.Error(t, err)

	_, err = FindComponentIn(nil, "anything")
	assert.Error(t, err)
}

func TestBuildStatus_Values(t *testing.T) {
	assert.Equal(t, BuildStatus("pending"), BuildStatusPending)
	assert.Equal(t, BuildStatus("running"), BuildStatusRunning)
	assert.Equal(t, BuildStatus("success"), BuildStatusSuccess)
	assert.Equal(t, BuildStatus("failed"), BuildStatusFailed)
	assert.Equal(t, BuildStatus("skipped"), BuildStatusSkipped)
}

func TestBuildPlan_LocalAssignments_WithLocalString(t *testing.T) {
	plan := BuildPlan{
		Assignments: []BuildAssignment{
			{
				Component: BuildComponent{Name: "catalog-api"},
				Host:      "local",
			},
			{
				Component: BuildComponent{Name: "catalog-web"},
				Host:      "remote-host",
			},
		},
	}

	local := plan.LocalAssignments()
	assert.Len(t, local, 1)
	assert.Equal(t, "catalog-api", local[0].Component.Name)

	remote := plan.RemoteAssignments()
	assert.Len(t, remote, 1)
	assert.Equal(t, "catalog-web", remote[0].Component.Name)
}

func TestBuildComponent_ResourceRequirements_JDKPriorityOverNPM(t *testing.T) {
	comp := BuildComponent{
		Name:   "android-app",
		HasNPM: true,
		HasJDK: true,
		HasGo:  true,
	}

	req := comp.ResourceRequirements()

	assert.Equal(t, float64(3), req.CPUCores)
	assert.Equal(t, scheduler.ContainerRequirements{
		Name:     "android-app",
		Image:    DefaultBuilderImage,
		CPUCores: 3,
		MemoryMB: 4096,
		DiskMB:   2048,
		Labels: map[string]string{
			"jdk": "true",
			"npm": "true",
		},
	}, req)
}

func TestBuildComponent_ResourceRequirements_CustomBuilderImage(t *testing.T) {
	comp := BuildComponent{
		Name:         "android-app",
		HasJDK:       true,
		BuilderImage: "registry.example.com/android-builder:1.2",
	}

	req := comp.ResourceRequirements()

	assert.Equal(t, "registry.example.com/android-builder:1.2", req.Image,
		"BuilderImage on the component must override DefaultBuilderImage")
}
