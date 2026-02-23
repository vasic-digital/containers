//go:build !integration

package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNerdctl_ParseVersion_ServerVersionSet exercises the branch in
// parseNerdctlVersion where Server.Version is non-empty, which causes
// the function to return the server version instead of the client version.
func TestNerdctl_ParseVersion_ServerVersionSet(t *testing.T) {
	// JSON with a different server version to ensure the server branch
	// is taken (not the client fallback).
	data := []byte(`{"Client":{"Version":"1.0.0"},"Server":{"Version":"2.5.3"}}`)
	ver, err := parseNerdctlVersion(data)
	require.NoError(t, err)
	assert.Equal(t, "2.5.3", ver,
		"parseNerdctlVersion should return Server.Version when set")
}

// TestNerdctl_ParseVersion_ServerEmpty exercises the fallback branch in
// parseNerdctlVersion where Server.Version is empty, which causes the
// function to return the client version.
func TestNerdctl_ParseVersion_ServerEmpty(t *testing.T) {
	data := []byte(`{"Client":{"Version":"1.2.0"},"Server":{"Version":""}}`)
	ver, err := parseNerdctlVersion(data)
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", ver,
		"parseNerdctlVersion should return Client.Version when Server.Version is empty")
}

// TestNerdctl_Stop_Error exercises the error branch in nerdctl Stop when
// the executor returns an error.
func TestNerdctl_Stop_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, name string, args ...string,
		) ([]byte, error) {
			return nil, fmt.Errorf("nerdctl stop failed: container not found")
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)

	err := n.Stop(context.Background(), "missing-ctr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nerdctl stop missing-ctr")
}

// TestNerdctl_List_ErrorBranch exercises the error return branch in List
// when the executor fails.
func TestNerdctl_List_ErrorBranch(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, name string, args ...string,
		) ([]byte, error) {
			return nil, fmt.Errorf("nerdctl ps: daemon not running")
		},
	}
	n := NewNerdctlRuntimeWithExecutor(exec)

	_, err := n.List(context.Background(), ListFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nerdctl ps")
}
