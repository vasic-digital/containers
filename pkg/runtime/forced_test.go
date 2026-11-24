//go:build !integration

package runtime

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestSupportedRunners_ComeFromTheModule(t *testing.T) {
	got := SupportedRunners()
	var want []string
	for _, rt := range defaultRuntimeFactory() {
		want = append(want, rt.Name())
	}
	assert.Equal(t, want, got, "supported list must be derived from the factory, not a second copy")
	assert.Contains(t, got, "podman")
	assert.Contains(t, got, "docker")
	assert.Contains(t, got, "nerdctl")
}

func TestParseRunnerFlag(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		val   string
		found bool
		rest  []string
		err   bool
	}{
		{"none", []string{"-v", "x"}, "", false, []string{"-v", "x"}, false},
		{"equals", []string{"--runner=docker", "a"}, "docker", true, []string{"a"}, false},
		{"space", []string{"a", "--runner", "podman", "b"}, "podman", true, []string{"a", "b"}, false},
		{"missing value", []string{"--runner"}, "", false, nil, true},
		{"empty value", []string{"--runner="}, "", false, nil, true},
		{"flag as value", []string{"--runner", "--other"}, "", false, nil, true},
		{"conflict", []string{"--runner=docker", "--runner=podman"}, "", false, nil, true},
		{"same twice ok", []string{"--runner=docker", "--runner", "docker"}, "docker", true, []string{}, false},
		{"after double dash untouched", []string{"--", "--runner=docker"}, "", false, []string{"--", "--runner=docker"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, rest, found, err := ParseRunnerFlag(c.args)
			if c.err {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrBadRunnerFlag))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.val, val)
			assert.Equal(t, c.found, found)
			assert.Equal(t, c.rest, rest)
		})
	}
}

func TestResolveRunner_Precedence(t *testing.T) {
	env := envOf(map[string]string{EnvRunner: "docker"})
	// flag beats env
	c, err := ResolveRunner("podman", true, env)
	require.NoError(t, err)
	assert.Equal(t, RunnerChoice{Name: "podman", Source: SourceFlag}, c)
	// env beats default
	c, err = ResolveRunner("", false, env)
	require.NoError(t, err)
	assert.Equal(t, RunnerChoice{Name: "docker", Source: SourceEnv}, c)
	// default
	c, err = ResolveRunner("", false, envOf(nil))
	require.NoError(t, err)
	assert.Equal(t, RunnerChoice{Name: "", Source: SourceDefault}, c)
	// auto == default order, whichever channel said it
	c, err = ResolveRunner("auto", true, env)
	require.NoError(t, err)
	assert.Equal(t, RunnerChoice{Name: "", Source: SourceFlag}, c)
	assert.False(t, c.Forced())
}

func TestResolveRunner_UnknownIsAnErrorEvenFromEnv(t *testing.T) {
	_, err := ResolveRunner("rkt", true, envOf(nil))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownRunner))
	assert.Contains(t, err.Error(), "podman", "error must list the supported runners")
	_, err = ResolveRunner("", false, envOf(map[string]string{EnvRunner: "Docker2"}))
	assert.True(t, errors.Is(err, ErrUnknownRunner))
	assert.Contains(t, err.Error(), EnvRunner)
}

func TestSelectRuntime_ForcedNeverFallsBack(t *testing.T) {
	old := detectFactory
	defer func() { detectFactory = old }()
	detectFactory = func() []ContainerRuntime {
		return []ContainerRuntime{
			&availableRuntime{name: "podman", available: true},
			&availableRuntime{name: "docker", available: false},
		}
	}
	rt, err := SelectRuntime(context.Background(), RunnerChoice{Name: "docker", Source: SourceFlag})
	assert.Nil(t, rt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRunnerUnavailable))
	assert.Contains(t, err.Error(), "docker")

	rt, err = SelectRuntime(context.Background(), RunnerChoice{Name: "podman", Source: SourceEnv})
	require.NoError(t, err)
	assert.Equal(t, "podman", rt.Name())

	// default order still works and is unchanged
	rt, err = SelectRuntime(context.Background(), RunnerChoice{Source: SourceDefault})
	require.NoError(t, err)
	assert.Equal(t, "podman", rt.Name())
}

// Real detection on this host: a forced runner whose binary is absent must be
// refused. Skips loudly when every supported client is installed.
func TestSelectRuntime_RealHost_AbsentRunnerRefused(t *testing.T) {
	var absent string
	for _, n := range SupportedRunners() {
		bin := n
		if n == "cri-o" {
			bin = "crictl"
		}
		if n == "kubernetes" {
			bin = "kubectl"
		}
		if _, err := exec.LookPath(bin); err != nil {
			absent = n
			break
		}
	}
	if absent == "" {
		t.Skip("every supported runner client is installed; cannot exercise the absent path on this host")
	}
	rt, err := SelectRuntime(context.Background(), RunnerChoice{Name: absent, Source: SourceFlag})
	assert.Nil(t, rt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRunnerUnavailable), err.Error())
	assert.True(t, strings.Contains(err.Error(), absent))
}

func TestSelectRuntime_RealHost_ForcedAvailableIsHonoured(t *testing.T) {
	ctx := context.Background()
	avail := DetectAll(ctx)
	if len(avail) == 0 {
		t.Skip("no container runtime on this host")
	}
	for _, a := range avail {
		rt, err := SelectRuntime(ctx, RunnerChoice{Name: a.Name(), Source: SourceFlag})
		require.NoError(t, err)
		assert.Equal(t, a.Name(), rt.Name())
	}
}
