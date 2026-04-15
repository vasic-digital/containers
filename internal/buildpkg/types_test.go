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
			name: "catalog-api is Go component",
			component: BuildComponent{
				Name:  "catalog-api",
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
			name: "catalogizer-androidtv is JDK component",
			component: BuildComponent{
				Name:   "catalogizer-androidtv",
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
			name: "catalogizer-desktop is Rust+NPM component",
			component: BuildComponent{
				Name:    "catalogizer-desktop",
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
			name: "catalog-web is NPM-only component",
			component: BuildComponent{
				Name:   "catalog-web",
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
			assert.Equal(t, "localhost/catalogizer-builder:latest", req.Image, "Image mismatch")
			assert.Equal(t, tt.component.Name, req.Name, "Name mismatch")
		})
	}
}

func TestBuildResult_Success(t *testing.T) {
	r := BuildResult{
		Component: "catalog-api",
		Status:    BuildStatusSuccess,
		Duration:  30,
	}
	assert.True(t, r.IsSuccess())
	assert.False(t, r.IsFailure())
}

func TestBuildResult_Failure(t *testing.T) {
	r := BuildResult{
		Component: "catalog-web",
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
				Component: BuildComponent{Name: "catalog-api"},
				Host:      "",
			},
			{
				Component: BuildComponent{Name: "catalog-web"},
				Host:      "build-server-1",
			},
			{
				Component: BuildComponent{Name: "catalogizer-android"},
				Host:      "build-server-2",
			},
		},
	}

	local := plan.LocalAssignments()
	assert.Len(t, local, 1)
	assert.Equal(t, "catalog-api", local[0].Component.Name)

	remote := plan.RemoteAssignments()
	assert.Len(t, remote, 2)

	byHost := plan.ByHost()
	assert.Len(t, byHost, 3)
	assert.Contains(t, byHost, "")
	assert.Contains(t, byHost, "build-server-1")
	assert.Contains(t, byHost, "build-server-2")
	assert.Len(t, byHost["build-server-1"], 1)
	assert.Equal(t, "catalog-web", byHost["build-server-1"][0].Component.Name)
}

func TestAllComponents(t *testing.T) {
	components := AllComponents()
	assert.Len(t, components, 7)

	names := make(map[string]bool)
	for _, c := range components {
		names[c.Name] = true
	}

	expected := []string{
		"catalog-api",
		"catalog-web",
		"catalogizer-api-client",
		"catalogizer-desktop",
		"installer-wizard",
		"catalogizer-android",
		"catalogizer-androidtv",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing component: %s", name)
	}
}

func TestFindComponent(t *testing.T) {
	api, err := FindComponent("catalog-api")
	assert.NoError(t, err)
	assert.Equal(t, "catalog-api", api.Name)
	assert.True(t, api.HasGo)

	_, err = FindComponent("nonexistent")
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
		Name:   "catalogizer-android",
		HasNPM: true,
		HasJDK: true,
		HasGo:  true,
	}

	req := comp.ResourceRequirements()

	assert.Equal(t, float64(3), req.CPUCores)
	assert.Equal(t, scheduler.ContainerRequirements{
		Name:     "catalogizer-android",
		Image:    "localhost/catalogizer-builder:latest",
		CPUCores: 3,
		MemoryMB: 4096,
		DiskMB:   2048,
		Labels: map[string]string{
			"jdk": "true",
			"npm": "true",
		},
	}, req)
}
