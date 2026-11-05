//go:build !integration

package buildpkg

import (
	"context"
	"errors"
	"testing"

	"digital.vasic.containers/pkg/remote"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LaunchLocalBuild is intentionally unsupported until the operator decides the
// open questions in docs/LOCAL_HOST_BUILD_TARGET_PROPOSAL.md. What this test
// pins is that the refusal is explicit and machine-detectable.
func TestLaunchLocalBuild_ReturnsSentinelError(t *testing.T) {
	e := NewBuildExecutor(nil, "/p", "/r")
	res, err := e.LaunchLocalBuild(context.Background(), remote.RemoteHost{}, "c", "1.0", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLocalBuildUnsupported))
	assert.Nil(t, res)
}

// A local assignment must never be silently dropped: it surfaces as a FAILED
// result carrying the reason, so the CLI exits non-zero.
func TestLocalAssignmentResults_AreFailedNotSilent(t *testing.T) {
	plan := BuildPlan{Assignments: []BuildAssignment{
		{Component: BuildComponent{Name: "a"}, Host: "local"},
		{Component: BuildComponent{Name: "b"}, Host: "h1"},
		{Component: BuildComponent{Name: "c"}, Host: ""},
	}}
	got := LocalAssignmentResults(plan)
	require.Len(t, got, 2)
	for _, r := range got {
		assert.True(t, r.IsFailure())
		assert.Equal(t, "local", r.Host)
		assert.Contains(t, r.Error, ErrLocalBuildUnsupported.Error())
	}
	assert.Equal(t, "a", got[0].Component)
	assert.Equal(t, "c", got[1].Component)
	assert.Empty(t, LocalAssignmentResults(BuildPlan{}))
}
