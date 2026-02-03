//go:build !integration

package runtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDockerRuntime_ImplementsInterface verifies that DockerRuntime
// satisfies the ContainerRuntime interface at compile time.
func TestDockerRuntime_ImplementsInterface(t *testing.T) {
	var _ ContainerRuntime = (*DockerRuntime)(nil)
	assert.NotNil(t, NewDockerRuntime())
}

// TestPodmanRuntime_ImplementsInterface verifies that PodmanRuntime
// satisfies the ContainerRuntime interface at compile time.
func TestPodmanRuntime_ImplementsInterface(t *testing.T) {
	var _ ContainerRuntime = (*PodmanRuntime)(nil)
	assert.NotNil(t, NewPodmanRuntime())
}

// TestKubernetesRuntime_ImplementsInterface verifies that
// KubernetesRuntime satisfies the ContainerRuntime interface.
func TestKubernetesRuntime_ImplementsInterface(t *testing.T) {
	var _ ContainerRuntime = (*KubernetesRuntime)(nil)
	assert.NotNil(t, NewKubernetesRuntime())
}

func TestContainerState_Values(t *testing.T) {
	tests := []struct {
		name  string
		state ContainerState
		want  string
	}{
		{"running", StateRunning, "running"},
		{"stopped", StateStopped, "stopped"},
		{"created", StateCreated, "created"},
		{"paused", StatePaused, "paused"},
		{"restarting", StateRestarting, "restarting"},
		{"removing", StateRemoving, "removing"},
		{"dead", StateDead, "dead"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.state))
		})
	}
}

func TestMapContainerState(t *testing.T) {
	tests := []struct {
		input string
		want  ContainerState
	}{
		{"running", StateRunning},
		{"Running", StateRunning},
		{"exited", StateStopped},
		{"stopped", StateStopped},
		{"created", StateCreated},
		{"paused", StatePaused},
		{"restarting", StateRestarting},
		{"removing", StateRemoving},
		{"dead", StateDead},
		{"unknown", ContainerState("unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapContainerState(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePercentage(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"0.50%", 0.50},
		{"100%", 100.0},
		{"12.34%", 12.34},
		{"0%", 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parsePercentage(tt.input)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestParseSizeToBytes(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"100B", 100},
		{"1kB", 1000},
		{"1KB", 1024},
		{"1MiB", 1024 * 1024},
		{"1MB", 1000 * 1000},
		{"2GiB", 2 * 1024 * 1024 * 1024},
		{"512", 512},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSizeToBytes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseLabelsString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "single label",
			input: "app=web",
			want:  map[string]string{"app": "web"},
		},
		{
			name:  "multiple labels",
			input: "app=web,env=prod,version=1.0",
			want: map[string]string{
				"app": "web", "env": "prod", "version": "1.0",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLabelsString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseMemUsage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantUsage uint64
		wantLimit uint64
	}{
		{
			name:      "standard format",
			input:     "100MiB / 1GiB",
			wantUsage: 100 * 1024 * 1024,
			wantLimit: 1024 * 1024 * 1024,
		},
		{
			name:      "no separator",
			input:     "invalid",
			wantUsage: 0,
			wantLimit: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, limit := parseMemUsage(tt.input)
			assert.Equal(t, tt.wantUsage, usage)
			assert.Equal(t, tt.wantLimit, limit)
		})
	}
}

func TestParseIOPair(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantA uint64
		wantB uint64
	}{
		{
			name:  "standard format",
			input: "1kB / 2kB",
			wantA: 1000,
			wantB: 2000,
		},
		{
			name:  "no separator",
			input: "invalid",
			wantA: 0,
			wantB: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := parseIOPair(tt.input)
			assert.Equal(t, tt.wantA, a)
			assert.Equal(t, tt.wantB, b)
		})
	}
}

func TestOptions_Start(t *testing.T) {
	opts := applyStartOptions([]StartOption{
		WithDetach(false),
		WithRemoveOnExit(true),
		WithEnv(map[string]string{"KEY": "VAL"}),
	})
	assert.False(t, opts.Detach)
	assert.True(t, opts.Remove)
	assert.Equal(t, "VAL", opts.Env["KEY"])
}

func TestOptions_Stop(t *testing.T) {
	opts := applyStopOptions(nil)
	assert.Equal(t, 10.0, opts.Timeout.Seconds())
}

func TestOptions_Remove(t *testing.T) {
	opts := applyRemoveOptions([]RemoveOption{
		WithForceRemove(true),
		WithRemoveVolumes(true),
	})
	assert.True(t, opts.Force)
	assert.True(t, opts.Volumes)
}

func TestOptions_Log(t *testing.T) {
	opts := applyLogOptions([]LogOption{
		WithFollow(true),
		WithSince("1h"),
		WithUntil("30m"),
		WithTail("100"),
	})
	assert.True(t, opts.Follow)
	assert.Equal(t, "1h", opts.Since)
	assert.Equal(t, "30m", opts.Until)
	assert.Equal(t, "100", opts.Tail)
}

func TestOptions_StopTimeout(t *testing.T) {
	opts := applyStopOptions([]StopOption{
		WithStopTimeout(30 * time.Second),
	})
	assert.Equal(t, 30*time.Second, opts.Timeout)
}

func TestOptions_LogDefaults(t *testing.T) {
	opts := applyLogOptions(nil)
	assert.False(t, opts.Follow)
	assert.Empty(t, opts.Since)
	assert.Empty(t, opts.Until)
	assert.Equal(t, "all", opts.Tail)
}
