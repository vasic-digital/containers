//go:build !integration

package compose

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeCandidatesForRunner(t *testing.T) {
	assert.Equal(t, [][]string{{"docker", "compose"}, {"docker-compose"}}, ComposeCandidatesForRunner("docker"))
	assert.Equal(t, [][]string{{"podman-compose"}, {"podman", "compose"}}, ComposeCandidatesForRunner("podman"))
	assert.Equal(t, [][]string{{"nerdctl", "compose"}}, ComposeCandidatesForRunner("nerdctl"))
	assert.Empty(t, ComposeCandidatesForRunner("lxd"))
	// never another runner's front-end
	for _, c := range ComposeCandidatesForRunner("podman") {
		assert.NotEqual(t, "docker", c[0])
	}
}

func TestDetectComposeCmdForRunner_NoComposeRunnerRefused(t *testing.T) {
	_, _, err := DetectComposeCmdForRunner("lxd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no compose implementation")
}

// Real host: docker is not installed here, so forcing it must be refused and
// must NOT pick podman's compose instead.
func TestDetectComposeCmdForRunner_RealHost_AbsentDockerRefused(t *testing.T) {
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("docker present on this host; absent path not exercisable") // SKIP-OK: host-capability guard; the opposite branch is exercised by its sibling test
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		t.Skip("docker-compose present on this host") // SKIP-OK: host-capability guard; the opposite branch is exercised by its sibling test
	}
	cmd, _, err := DetectComposeCmdForRunner("docker")
	require.Error(t, err)
	assert.Empty(t, cmd)
	assert.Contains(t, err.Error(), "refusing to fall back")
	_, err = NewDefaultOrchestratorForRunner(t.TempDir(), nil, "docker")
	require.Error(t, err)
}
