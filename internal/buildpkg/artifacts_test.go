package buildpkg

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.containers/pkg/remote"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactCollector_DiscoverArtifacts(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = true
	mock.executeResult = &remote.CommandResult{
		Stdout:   "/tmp/catalogizer-build/releases/catalog-api/linux-amd64/v2.2.0-build.19/BUILD_INFO.json\n/tmp/catalogizer-build/releases/catalog-api/linux-arm64/v2.2.0-build.19/BUILD_INFO.json\n",
		ExitCode: 0,
	}

	collector := NewArtifactCollector(mock, "/home/user/Catalogizer", "/tmp/catalogizer-build")
	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}

	paths, err := collector.DiscoverArtifacts(context.Background(), host, "catalog-api", "v2.2.0-build.19")
	require.NoError(t, err)
	require.Len(t, paths, 2)

	assert.Equal(t, "/tmp/catalogizer-build/releases/catalog-api/linux-amd64/v2.2.0-build.19", paths[0])
	assert.Equal(t, "/tmp/catalogizer-build/releases/catalog-api/linux-arm64/v2.2.0-build.19", paths[1])

	require.Len(t, mock.executedCommands, 1)
	assert.Contains(t, mock.executedCommands[0].command, "BUILD_INFO.json")
	assert.Contains(t, mock.executedCommands[0].command, "v2.2.0-build.19")
	assert.Contains(t, mock.executedCommands[0].command, "catalog-api")
}

func TestArtifactCollector_DiscoverArtifactsEmptyResult(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = true
	mock.executeResult = &remote.CommandResult{
		Stdout:   "",
		ExitCode: 0,
	}

	collector := NewArtifactCollector(mock, "/home/user/Catalogizer", "/tmp/catalogizer-build")
	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}

	paths, err := collector.DiscoverArtifacts(context.Background(), host, "catalog-api", "v2.2.0")
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestArtifactCollector_CollectArtifacts(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = true

	collector := NewArtifactCollector(mock, "/home/user/Catalogizer", "/tmp/catalogizer-build")
	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}

	remotePaths := []string{
		"/tmp/catalogizer-build/releases/catalog-api/linux-amd64/v2.2.0-build.19",
		"/tmp/catalogizer-build/releases/catalog-api/linux-arm64/v2.2.0-build.19",
	}

	err := collector.CollectArtifacts(context.Background(), host, remotePaths)
	require.NoError(t, err)

	require.Len(t, mock.copiedFiles, 2)

	assert.Equal(t, "/tmp/catalogizer-build/releases/catalog-api/linux-amd64/v2.2.0-build.19", mock.copiedFiles[0].localDir)
	assert.True(t, strings.HasSuffix(mock.copiedFiles[0].remoteDir, "catalog-api/linux-amd64/v2.2.0-build.19"))

	assert.Equal(t, "/tmp/catalogizer-build/releases/catalog-api/linux-arm64/v2.2.0-build.19", mock.copiedFiles[1].localDir)
	assert.True(t, strings.HasSuffix(mock.copiedFiles[1].remoteDir, "catalog-api/linux-arm64/v2.2.0-build.19"))
}

func TestArtifactCollector_CollectFromUnreachableHost(t *testing.T) {
	mock := newMockExecutor()
	mock.reachable["thinker"] = false

	collector := NewArtifactCollector(mock, "/home/user/Catalogizer", "/tmp/catalogizer-build")
	host := remote.RemoteHost{Name: "thinker", Address: "thinker.local"}

	_, err := collector.DiscoverArtifacts(context.Background(), host, "catalog-api", "v2.2.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not reachable")
	assert.Contains(t, err.Error(), "thinker")

	err = collector.CollectArtifacts(context.Background(), host, []string{"/tmp/some/path"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not reachable")
}
