package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatusOutput_ValidLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ServiceStatus
	}{
		{
			name:  "single service running",
			input: "web|running|healthy|0.0.0.0:8080->80/tcp|0\n",
			expected: []ServiceStatus{
				{
					Name:     "web",
					State:    "running",
					Health:   "healthy",
					Ports:    []string{"0.0.0.0:8080->80/tcp"},
					ExitCode: 0,
				},
			},
		},
		{
			name: "multiple services",
			input: "web|running|healthy|0.0.0.0:8080->80/tcp|0\n" +
				"db|running|healthy|0.0.0.0:5432->5432/tcp|0\n" +
				"redis|exited||0.0.0.0:6379->6379/tcp|1\n",
			expected: []ServiceStatus{
				{
					Name: "web", State: "running",
					Health: "healthy",
					Ports:  []string{"0.0.0.0:8080->80/tcp"},
				},
				{
					Name: "db", State: "running",
					Health: "healthy",
					Ports:  []string{"0.0.0.0:5432->5432/tcp"},
				},
				{
					Name: "redis", State: "exited",
					Ports:    []string{"0.0.0.0:6379->6379/tcp"},
					ExitCode: 1,
				},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   \n   \n",
			expected: nil,
		},
		{
			name: "multiple ports",
			input: "app|running|healthy|" +
				"0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp|0\n",
			expected: []ServiceStatus{
				{
					Name: "app", State: "running",
					Health: "healthy",
					Ports: []string{
						"0.0.0.0:80->80/tcp",
						"0.0.0.0:443->443/tcp",
					},
				},
			},
		},
		{
			name:  "no ports",
			input: "worker|running|||0\n",
			expected: []ServiceStatus{
				{
					Name: "worker", State: "running",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStatusOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single port",
			input:    "0.0.0.0:8080->80/tcp",
			expected: []string{"0.0.0.0:8080->80/tcp"},
		},
		{
			name:  "multiple ports",
			input: "0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp",
			expected: []string{
				"0.0.0.0:80->80/tcp",
				"0.0.0.0:443->443/tcp",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePorts(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProjectArgs(t *testing.T) {
	o := NewOrchestrator("docker", []string{"compose"}, "/tmp", nil)

	tests := []struct {
		name     string
		project  ComposeProject
		expected []string
	}{
		{
			name:     "empty project",
			project:  ComposeProject{},
			expected: nil,
		},
		{
			name: "file only",
			project: ComposeProject{
				File: "docker-compose.yml",
			},
			expected: []string{"-f", "docker-compose.yml"},
		},
		{
			name: "all fields",
			project: ComposeProject{
				Name:    "myproject",
				File:    "compose.yaml",
				Profile: "dev",
			},
			expected: []string{
				"-f", "compose.yaml",
				"--project-name", "myproject",
				"--profile", "dev",
			},
		},
		{
			name: "name and profile only",
			project: ComposeProject{
				Name:    "proj",
				Profile: "test",
			},
			expected: []string{
				"--project-name", "proj",
				"--profile", "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := o.projectArgs(tt.project)
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestNewOrchestrator(t *testing.T) {
	o := NewOrchestrator(
		"podman", []string{"compose"}, "/var/lib", nil,
	)
	require.NotNil(t, o)
	assert.Equal(t, "podman", o.composeCmd)
	assert.Equal(t, []string{"compose"}, o.composeArgs)
	assert.Equal(t, "/var/lib", o.workDir)
	assert.NotNil(t, o.logger)
}

func TestUpOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []UpOption
		expected upOptions
	}{
		{
			name: "defaults",
			opts: nil,
			expected: upOptions{
				Detach: true,
			},
		},
		{
			name: "all options set",
			opts: []UpOption{
				WithUpDetach(false),
				WithRemoveOrphans(true),
				WithBuildFirst(true),
				WithForceRecreate(true),
				WithNoRecreate(false),
				WithUpTimeout(30),
				WithWait(true),
			},
			expected: upOptions{
				Detach:        false,
				RemoveOrphans: true,
				BuildFirst:    true,
				ForceRecreate: true,
				NoRecreate:    false,
				Timeout:       30,
				Wait:          true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyUpOptions(tt.opts)
			assert.Equal(t, tt.expected.Detach, cfg.Detach)
			assert.Equal(t, tt.expected.RemoveOrphans,
				cfg.RemoveOrphans)
			assert.Equal(t, tt.expected.BuildFirst,
				cfg.BuildFirst)
			assert.Equal(t, tt.expected.ForceRecreate,
				cfg.ForceRecreate)
			assert.Equal(t, tt.expected.Timeout, cfg.Timeout)
			assert.Equal(t, tt.expected.Wait, cfg.Wait)
		})
	}
}

func TestDownOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []DownOption
		expected downOptions
	}{
		{
			name:     "defaults",
			opts:     nil,
			expected: downOptions{},
		},
		{
			name: "all options set",
			opts: []DownOption{
				WithDownRemoveOrphans(true),
				WithDownRemoveVolumes(true),
				WithDownRemoveImages("all"),
				WithDownTimeout(60),
			},
			expected: downOptions{
				RemoveOrphans: true,
				RemoveVolumes: true,
				RemoveImages:  "all",
				Timeout:       60,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyDownOptions(tt.opts)
			assert.Equal(t, tt.expected.RemoveOrphans,
				cfg.RemoveOrphans)
			assert.Equal(t, tt.expected.RemoveVolumes,
				cfg.RemoveVolumes)
			assert.Equal(t, tt.expected.RemoveImages,
				cfg.RemoveImages)
			assert.Equal(t, tt.expected.Timeout, cfg.Timeout)
		})
	}
}
