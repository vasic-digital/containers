// SPDX-License-Identifier: Apache-2.0
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

// TestWebWasm_HappyPath: end-to-end orchestration against an injected
// runner. Verifies Selector→Backend→ContainerRunner wiring and the
// CONST-039 evidence layer (index.html + *.wasm + *.js all present +
// non-empty in the output directory).
func TestWebWasm_HappyPath(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	// Simulate the output directory that wasmJsBrowserDistribution
	// creates inside the container's bind-mounted source tree.
	wasmOutputDir := filepath.Join(srcDir,
		"webApp/build/dist/wasmJs/productionExecutable")
	require.NoError(t, os.MkdirAll(wasmOutputDir, 0o755))

	// Primary artifact that OutputSubpath points at.
	primaryPath := filepath.Join(wasmOutputDir, "index.html")

	runner := &fakeContainerRunner{
		imageExists: true,
		exitCode:    0,
		stdout:      "BUILD SUCCESSFUL\nwasmJsBrowserDistribution completed\n",
		// The fake runner creates the primary artifact plus the full
		// CONST-039 evidence set when Run() is called.
		producesPath:    primaryPath,
		producesContent: []byte("<html>Wasm bundle root</html>"),
	}

	// Pre-create the sibling evidence files that the real KGP produces.
	// The fake runner only writes producesPath; we create the others
	// here to exercise verifyWasmOutputs independently.
	require.NoError(t, os.WriteFile(
		filepath.Join(wasmOutputDir, "app.wasm"),
		bytes.Repeat([]byte("W"), 1024), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(wasmOutputDir, "app.js"),
		bytes.Repeat([]byte("J"), 512), 0o644))

	b := newWebWasmContainerBackendWithRunner("test/web-wasm-image:jdk21", nil, runner)

	subpath := "webApp/build/dist/wasmJs/productionExecutable/index.html"
	result := b.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "js", Arch: "wasm"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :webApp:wasmJsBrowserDistribution",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
		Timeout:       1 * time.Minute,
	})

	// CONST-039 runtime evidence: result must carry a real artifact.
	require.NoError(t, result.Error, "web-wasm happy path must not error")
	assert.Equal(t, "web-wasm-container", result.BackendName)
	assert.True(t, runner.runCalled, "container runner must have been invoked")
	assert.Greater(t, result.ArtifactSize, int64(0), "artifact must have non-zero size")

	// Verify the artifact was copied to HostOutputDir.
	stat, err := os.Stat(result.ArtifactPath)
	require.NoError(t, err, "artifact must exist on host after build")
	assert.Equal(t, result.ArtifactSize, stat.Size(),
		"ArtifactSize in result must match stat on disk")

	// Verify CONST-039 evidence: all Wasm outputs present.
	assert.Contains(t, runner.gotSpec.Command, "wasmJsBrowserDistribution",
		"build command must include the Wasm distribution task")
}

// TestWebWasm_MissingImageReturnsHonestError: SKELETON-state honesty.
// When the container image is absent, Build() returns an error that
// names the provisioning doc + the SKIP-OK ticket — not a bluff.
func TestWebWasm_MissingImageReturnsHonestError(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	runner := &fakeContainerRunner{imageExists: false}
	b := newWebWasmContainerBackendWithRunner("ghcr.io/example/missing-wasm:jdk21", nil, runner)

	result := b.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "js", Arch: "wasm"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :webApp:wasmJsBrowserDistribution",
		OutputSubpath: "webApp/build/dist/wasmJs/productionExecutable/index.html",
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not present on host")
	assert.Contains(t, result.Error.Error(), "web-wasm-image-provisioning")
	assert.Contains(t, result.Error.Error(), "#crossbuild-web-wasm-image-provisioning")
	assert.False(t, runner.runCalled, "runner must not be invoked when image is absent")
}

// TestWebWasm_MissingPrimaryArtifactCitesKGPGap: anti-bluff + honest
// disclosure. When the container exits 0 but the primary artifact is
// missing (symptom of #wasmjs-production-distribution-gap), the error
// message must cite the gap ticket so the operator knows what to fix.
func TestWebWasm_MissingPrimaryArtifactCitesKGPGap(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	runner := &fakeContainerRunner{
		imageExists: true,
		exitCode:    0,
		stdout:      "BUILD SUCCESSFUL\n",
		// producesPath intentionally empty — simulates KGP gap where
		// output is NOT written even though Gradle exits 0.
	}
	b := newWebWasmContainerBackendWithRunner("test/web-wasm-image:jdk21", nil, runner)

	result := b.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "js", Arch: "wasm"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :webApp:wasmJsBrowserDistribution",
		OutputSubpath: "webApp/build/dist/wasmJs/productionExecutable/index.html",
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "#wasmjs-production-distribution-gap",
		"error must cite the upstream KGP gap ticket")
	assert.True(t, runner.runCalled, "runner must have been called before detecting artifact absence")
}

// TestWebWasm_ZeroByteArtifactIsBluff: zero-byte primary artifact
// must be rejected, not silently accepted as a valid build result.
func TestWebWasm_ZeroByteArtifactIsBluff(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	subpath := "webApp/build/dist/wasmJs/productionExecutable/index.html"
	runner := &fakeContainerRunner{
		imageExists:     true,
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: []byte{}, // zero bytes — bluff
	}
	b := newWebWasmContainerBackendWithRunner("test/web-wasm-image:jdk21", nil, runner)

	result := b.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "js", Arch: "wasm"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :webApp:wasmJsBrowserDistribution",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "zero-byte")
}

// TestWebWasm_Capabilities: verify the backend's declared capabilities
// are correct — js/wasm target, no host restriction.
func TestWebWasm_Capabilities(t *testing.T) {
	b := NewWebWasmContainerBackend("", nil)
	caps := b.Capabilities()

	require.Len(t, caps.SupportsTargets, 1)
	assert.Equal(t, "js", caps.SupportsTargets[0].OS)
	assert.Equal(t, "wasm", caps.SupportsTargets[0].Arch)
	assert.Empty(t, caps.RequiresHostOS, "web-wasm backend must run on any host OS")
	assert.True(t, caps.IsolatesEnvironment)
}

// TestWebWasm_MissingWasmEvidence: even when the primary artifact
// exists, absence of the .wasm sidecar file must be detected and
// reported (CONST-039 evidence check).
func TestWebWasm_MissingWasmEvidence(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	wasmOutputDir := filepath.Join(srcDir,
		"webApp/build/dist/wasmJs/productionExecutable")
	require.NoError(t, os.MkdirAll(wasmOutputDir, 0o755))

	primaryPath := filepath.Join(wasmOutputDir, "index.html")
	runner := &fakeContainerRunner{
		imageExists:     true,
		exitCode:        0,
		stdout:          "BUILD SUCCESSFUL\n",
		producesPath:    primaryPath,
		producesContent: []byte("<html>ok</html>"),
	}
	// Only index.html is produced; .wasm and .js are absent.
	// This simulates a partial/broken KGP output.

	b := newWebWasmContainerBackendWithRunner("test/web-wasm-image:jdk21", nil, runner)

	result := b.Build(context.Background(), BuildRequest{
		Target:        Target{OS: "js", Arch: "wasm"},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :webApp:wasmJsBrowserDistribution",
		OutputSubpath: "webApp/build/dist/wasmJs/productionExecutable/index.html",
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "CONST-039 evidence check failed",
		"error must name the CONST-039 check that caught the incomplete bundle")
	assert.Contains(t, result.Error.Error(), "#wasmjs-production-distribution-gap")
}

// TestVerifyWasmOutputs_AllPresent: unit test for the glob-based
// evidence verifier with all expected files present.
func TestVerifyWasmOutputs_AllPresent(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"index.html", "app.wasm", "bundle.js"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("content"), 0o644))
	}
	missing := verifyWasmOutputs(dir, defaultWasmOutputs())
	assert.Empty(t, missing, "all default wasm outputs present — none should be missing")
}

// TestVerifyWasmOutputs_MissingWasm: unit test verifying that absence
// of a .wasm file is correctly detected even when .html and .js exist.
func TestVerifyWasmOutputs_MissingWasm(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("h"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.js"), []byte("j"), 0o644))
	// No .wasm file.

	missing := verifyWasmOutputs(dir, defaultWasmOutputs())
	assert.Contains(t, missing, "*.wasm",
		"*.wasm pattern must be in the missing list when no .wasm file exists")
}
