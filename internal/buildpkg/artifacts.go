package buildpkg

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"digital.vasic.containers/pkg/remote"
)

type ArtifactCollector struct {
	executor   RemoteExecutor
	projectDir string
	remoteDir  string
}

func NewArtifactCollector(executor RemoteExecutor, projectDir, remoteDir string) *ArtifactCollector {
	return &ArtifactCollector{
		executor:   executor,
		projectDir: projectDir,
		remoteDir:  remoteDir,
	}
}

func (c *ArtifactCollector) DiscoverArtifacts(ctx context.Context, host remote.RemoteHost, component, versionString string) ([]string, error) {
	if !c.executor.IsReachable(ctx, host) {
		return nil, fmt.Errorf("host %s is not reachable", host.Name)
	}

	findCmd := fmt.Sprintf("find %s/releases/%s -name 'BUILD_INFO.json' -path '*%s*'", c.remoteDir, component, versionString)

	result, err := c.executor.Execute(ctx, host, findCmd)
	if err != nil {
		return nil, fmt.Errorf("discover artifacts: %w", err)
	}

	var paths []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, filepath.Dir(line))
	}

	return paths, nil
}

func (c *ArtifactCollector) CollectArtifacts(ctx context.Context, host remote.RemoteHost, remotePaths []string) error {
	if !c.executor.IsReachable(ctx, host) {
		return fmt.Errorf("host %s is not reachable", host.Name)
	}

	for _, rp := range remotePaths {
		relPath := strings.TrimPrefix(rp, c.remoteDir+"/")
		if relPath == rp {
			relPath = filepath.Base(rp)
		}

		localPath := filepath.Join(c.projectDir, relPath)

		err := c.executor.CopyDir(ctx, host, rp, localPath)
		if err != nil {
			return fmt.Errorf("collect artifact %s: %w", rp, err)
		}
	}

	return nil
}
