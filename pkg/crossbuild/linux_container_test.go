package crossbuild

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinuxContainer_HappyPath: end-to-end orchestration against an
// injected runner. Mirrors TestWineContainer_HappyPath but for the
// Linux target — proves the parallel code path produces positive
// runtime evidence.
func TestLinuxContainer_HappyPath(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "desktopApp/build/compose/binaries/main-release/deb/yole_1.0.1-1_amd64.deb"

	runner := &fakeContainerRunner{
		imageExists:     true,
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: bytes.Repeat([]byte("D"), 512),
		stdout:          "BUILD SUCCESSFUL\n",
	}
	l := newLinuxContainerBackendWithRunner("test/linux-image", "amd64", runner)

	result := l.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "linux", Arch: "amd64"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :desktopApp:packageReleaseDeb",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
		Timeout:       1 * time.Minute,
	})

	require.NoError(t, result.Error)
	assert.Equal(t, "linux-container", result.BackendName)
	assert.True(t, runner.runCalled)
	assert.Equal(t, int64(512), result.ArtifactSize)
	assert.Contains(t, runner.gotSpec.Command, "packageReleaseDeb")
	stat, err := os.Stat(result.ArtifactPath)
	require.NoError(t, err)
	assert.Equal(t, int64(512), stat.Size())
}

// TestLinuxContainer_MissingImageReturnsHonestError: SKELETON-state
// honesty — when the image is not on the host, Build() points the
// operator at the provisioning doc + the SKIP-OK ticket.
func TestLinuxContainer_MissingImageReturnsHonestError(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	runner := &fakeContainerRunner{imageExists: false}
	l := newLinuxContainerBackendWithRunner("ghcr.io/example/missing-linux:tag", "amd64", runner)

	result := l.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "linux", Arch: "amd64"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :desktopApp:packageReleaseDeb",
		OutputSubpath: "out.deb",
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not present on host")
	assert.Contains(t, result.Error.Error(), "linux-image-provisioning")
	assert.Contains(t, result.Error.Error(), "#crossbuild-linux-image-provisioning")
	assert.False(t, runner.runCalled)
}

// TestLinuxContainer_ZeroByteArtifactIsBluff: anti-bluff invariant.
func TestLinuxContainer_ZeroByteArtifactIsBluff(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "build/empty.deb"

	runner := &fakeContainerRunner{
		imageExists:     true,
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: []byte{},
	}
	l := newLinuxContainerBackendWithRunner("test/linux-image", "amd64", runner)

	result := l.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "linux", Arch: "amd64"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew assemble",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "zero-byte artifact")
}

// TestLinuxContainer_CapabilitiesHonest: locks the capability
// advertisement. Linux container backend MUST NOT advertise
// RequiresHostOS (works on every host) AND MUST advertise the arch
// honestly so the Selector routes correctly on Apple Silicon
// (arm64) vs x86 hosts.
func TestLinuxContainer_CapabilitiesHonest(t *testing.T) {
	t.Run("amd64", func(t *testing.T) {
		l := NewLinuxContainerBackend("", "amd64")
		caps := l.Capabilities()
		assert.Empty(t, caps.RequiresHostOS,
			"linux-container MUST work on every host OS")
		assert.Equal(t, []Target{{OS: "linux", Arch: "amd64"}}, caps.SupportsTargets)
		assert.True(t, caps.IsolatesEnvironment)
	})
	t.Run("arm64", func(t *testing.T) {
		l := NewLinuxContainerBackend("", "arm64")
		caps := l.Capabilities()
		assert.Equal(t, []Target{{OS: "linux", Arch: "arm64"}}, caps.SupportsTargets,
			"arm64 backend MUST advertise arm64, not amd64 (anti-bluff)")
	})
}

// TestLinuxContainer_DefaultImageRefByArch verifies the default tag
// suffix reflects the chosen architecture, so an Apple Silicon
// developer doesn't accidentally pull the amd64 image and pay
// emulation costs.
func TestLinuxContainer_DefaultImageRefByArch(t *testing.T) {
	l := NewLinuxContainerBackend("", "amd64")
	assert.Equal(t, "ghcr.io/vasic-digital/crossbuild-linux:jdk17-amd64", l.imageRef)

	l2 := NewLinuxContainerBackend("", "arm64")
	assert.Equal(t, "ghcr.io/vasic-digital/crossbuild-linux:jdk17-arm64", l2.imageRef)

	l3 := NewLinuxContainerBackend("custom/tag", "amd64")
	assert.Equal(t, "custom/tag", l3.imageRef, "explicit imageRef takes precedence over arch default")
}
