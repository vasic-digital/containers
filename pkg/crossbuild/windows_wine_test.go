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

// fakeContainerRunner lets tests exercise the wine-container
// orchestration without touching podman/docker. The test controls
// whether the image "exists" + what the container "produces".
type fakeContainerRunner struct {
	imageExists     bool
	exitCode        int
	errOnRun        error
	stdout          string
	stderr          string
	producesPath    string
	producesContent []byte
	gotSpec         containerRunSpec
	runCalled       bool
}

func (f *fakeContainerRunner) ImageExists(_ context.Context, _ string) bool {
	return f.imageExists
}

func (f *fakeContainerRunner) Run(_ context.Context, spec containerRunSpec) (int, error) {
	f.runCalled = true
	f.gotSpec = spec
	spec.Stdout.WriteString(f.stdout)
	spec.Stderr.WriteString(f.stderr)
	if f.producesPath != "" {
		if err := os.MkdirAll(filepath.Dir(f.producesPath), 0o755); err != nil {
			return -1, err
		}
		if err := os.WriteFile(f.producesPath, f.producesContent, 0o644); err != nil {
			return -1, err
		}
	}
	return f.exitCode, f.errOnRun
}

// TestWineContainer_HappyPath verifies the orchestration end-to-end
// against an injected runner: image exists → run exits 0 → artifact
// produced → copied to HostOutputDir with correct size.
func TestWineContainer_HappyPath(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "desktopApp/build/compose/binaries/main-release/msi/Yole-1.0.1.msi"

	runner := &fakeContainerRunner{
		imageExists:     true,
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: bytes.Repeat([]byte("M"), 256), // pretend 256-byte MSI
		stdout:          "Gradle build successful\n",
	}
	w := newWineContainerBackendWithRunner("test/image:latest", runner)

	result := w.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "windows", Arch: "amd64"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :desktopApp:packageReleaseMsi",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
		Timeout:       1 * time.Minute,
	})

	require.NoError(t, result.Error)
	assert.Equal(t, "wine-container", result.BackendName)
	assert.True(t, runner.runCalled)
	assert.Equal(t, int64(256), result.ArtifactSize)
	assert.Equal(t, "test/image:latest", runner.gotSpec.Image)
	assert.Equal(t, srcDir, runner.gotSpec.MountSource)
	assert.Equal(t, "/work/src", runner.gotSpec.MountTarget)
	assert.Contains(t, runner.gotSpec.Command, "packageReleaseMsi")

	// Positive runtime evidence: artifact actually exists at the
	// reported path.
	stat, err := os.Stat(result.ArtifactPath)
	require.NoError(t, err)
	assert.Equal(t, int64(256), stat.Size())
}

// TestWineContainer_MissingImageReturnsHonestError verifies the
// SKELETON-state guarantee: when the operator has not yet
// provisioned the image, Build() returns an actionable error
// pointing at the provisioning docs + the skip-marker.
func TestWineContainer_MissingImageReturnsHonestError(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	runner := &fakeContainerRunner{imageExists: false}
	w := newWineContainerBackendWithRunner("ghcr.io/example/missing:tag", runner)

	result := w.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "windows", Arch: "amd64"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :desktopApp:packageReleaseMsi",
		OutputSubpath: "out.msi",
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not present on host")
	assert.Contains(t, result.Error.Error(), "windows-image-provisioning",
		"error MUST direct the operator to the provisioning docs")
	assert.Contains(t, result.Error.Error(), "#crossbuild-windows-image-provisioning",
		"error MUST reference the skip-OK ticket so a green CI behind the gate is honest")
	assert.False(t, runner.runCalled, "Run must not be invoked when image is missing")
}

// TestWineContainer_ZeroByteArtifactIsBluff covers the
// container-side equivalent of TestHostDirect_RejectsZeroByteArtifact.
func TestWineContainer_ZeroByteArtifactIsBluff(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "build/empty.msi"

	runner := &fakeContainerRunner{
		imageExists:     true,
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: []byte{},
	}
	w := newWineContainerBackendWithRunner("test/image", runner)

	result := w.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "windows", Arch: "amd64"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew assemble",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "zero-byte artifact")
	assert.Contains(t, result.Error.Error(), "bluff")
}

// TestWineContainer_CapabilitiesRestrictsToLinuxHost asserts the
// honesty of the Capabilities() advertisement: this backend MUST
// declare RequiresHostOS=[linux] so the Selector doesn't route a
// macOS-host request to it.
func TestWineContainer_CapabilitiesRestrictsToLinuxHost(t *testing.T) {
	w := NewWineContainerBackend("")
	caps := w.Capabilities()
	assert.Equal(t, []string{"linux"}, caps.RequiresHostOS,
		"wine-container MUST advertise RequiresHostOS=[linux] — Wine in macOS containers is unstable")
	assert.Equal(t, []Target{{OS: "windows", Arch: "amd64"}}, caps.SupportsTargets)
	assert.True(t, caps.IsolatesEnvironment,
		"container backends MUST isolate environment from host")
}

// TestWineContainer_DefaultImageRef locks the default registry path
// so a typo in the production constructor is caught.
func TestWineContainer_DefaultImageRef(t *testing.T) {
	w := NewWineContainerBackend("")
	assert.Equal(t, "ghcr.io/vasic-digital/crossbuild-wine:latest", w.imageRef)
	w2 := NewWineContainerBackend("custom/image:v1")
	assert.Equal(t, "custom/image:v1", w2.imageRef)
}
