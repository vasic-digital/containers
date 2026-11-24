//go:build !integration

package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func do(env map[string]string, args ...string) (int, string, string) {
	var o, e bytes.Buffer
	rc := run(args, &o, &e, func(k string) string { return env[k] })
	return rc, o.String(), e.String()
}

func TestList(t *testing.T) {
	rc, out, _ := do(nil, "list")
	assert.Equal(t, 0, rc)
	assert.Contains(t, strings.Fields(out), "podman")
	assert.Contains(t, strings.Fields(out), "nerdctl")
}

func TestUnknownRunnerIsRC2(t *testing.T) {
	rc, _, errs := do(nil, "resolve", "--runner=rkt")
	assert.Equal(t, 2, rc)
	assert.Contains(t, errs, "supported runners are")
	rc, _, _ = do(map[string]string{"CONTAINERS_RUNNER": "bogus"}, "resolve")
	assert.Equal(t, 2, rc)
}

func TestRealHost_ForcedAbsentRunnerIsRC2_NoFallback(t *testing.T) {
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("docker installed here")
	}
	rc, out, errs := do(nil, "resolve", "--runner=docker")
	assert.Equal(t, 2, rc)
	assert.Empty(t, out, "must not print a fallback runner")
	assert.Contains(t, errs, "refusing to fall back")
	rc, out, _ = do(map[string]string{"CONTAINERS_RUNNER": "docker"}, "resolve")
	assert.Equal(t, 2, rc)
	assert.Empty(t, out)
}

func TestRealHost_PodmanForcedAndPrecedence(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("no podman")
	}
	rc, out, _ := do(nil, "resolve", "--runner", "podman")
	assert.Equal(t, 0, rc)
	assert.Equal(t, "podman\tflag\tforced\n", out)
	// flag beats an env that would otherwise be refused
	rc, out, _ = do(map[string]string{"CONTAINERS_RUNNER": "docker"}, "resolve", "--runner=podman")
	assert.Equal(t, 0, rc)
	assert.Equal(t, "podman\tflag\tforced\n", out)
	rc, out, _ = do(map[string]string{"CONTAINERS_RUNNER": "podman"}, "resolve")
	assert.Equal(t, "podman\tenv\tforced\n", out)
	rc, out, _ = do(nil, "resolve")
	assert.Equal(t, 0, rc)
	assert.True(t, strings.HasSuffix(out, "\tdefault\tauto\n"), out)
}

func TestRealHost_AutoIsNotForced(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("no podman")
	}
	rc, out, _ := do(map[string]string{"CONTAINERS_RUNNER": "docker"}, "resolve", "--runner=auto")
	assert.Equal(t, 0, rc)
	assert.True(t, strings.HasSuffix(out, "\tflag\tauto\n"), out)
}
