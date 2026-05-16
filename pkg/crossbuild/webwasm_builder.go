// SPDX-License-Identifier: Apache-2.0
package crossbuild

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WebWasmContainerBackend cross-builds WebAssembly / Wasm-JS browser
// distribution bundles by executing the BuildCommand inside a Linux
// container that has JDK 21 + a pinned Gradle wrapper pre-installed.
//
// # Upstream constraint (honest disclosure, CONST-039)
//
// Kotlin/Wasm production distribution (`wasmJsBrowserDistribution`) is
// gated on a KGP (Kotlin Gradle Plugin) fix that was not yet merged as
// of Kotlin 2.0.20 / KGP 2.0.20 (tracked internally as
// #wasmjs-production-distribution-gap). On an affected KGP version
// the Gradle task exits 0 but does NOT copy the output to
// `build/dist/wasmJs/productionExecutable/`. The backend detects this
// by verifying that expected output files exist AND are non-empty; if
// they are absent it returns an explicit, honest error citing the gap
// ticket rather than claiming BUILD SUCCESSFUL.
//
// When the upstream KGP fix lands and the consumer project adopts it,
// this backend will succeed end-to-end without any code change here.
//
// # Operator provisioning
//
// Build the container image once on any host with rootless podman/docker:
//
//	podman build \
//	    -t ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21 \
//	    -f web_wasm.Containerfile \
//	    pkg/crossbuild/
//
// Verify:
//
//	podman run --rm ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21 \
//	    gradle --version
//
// Once the image is on the host, WebWasmContainerBackend.Build()
// succeeds end-to-end (assuming the consumer project's KGP is fixed).
type WebWasmContainerBackend struct {
	imageRef        string
	expectedOutputs []string // relative to HostOutputDir; checked post-build
	runner          containerRunner
}

// NewWebWasmContainerBackend returns the production backend. imageRef
// defaults to "ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21" when
// empty. expectedOutputs defaults to the canonical Wasm JS distribution
// file set (index.html + .wasm + .js); pass a non-nil slice to override
// for a custom consumer project layout.
func NewWebWasmContainerBackend(imageRef string, expectedOutputs []string) *WebWasmContainerBackend {
	if imageRef == "" {
		imageRef = "ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21"
	}
	if expectedOutputs == nil {
		expectedOutputs = defaultWasmOutputs()
	}
	return &WebWasmContainerBackend{
		imageRef:        imageRef,
		expectedOutputs: expectedOutputs,
		runner:          realContainerRunner{},
	}
}

// newWebWasmContainerBackendWithRunner is the test seam.
func newWebWasmContainerBackendWithRunner(imageRef string, expectedOutputs []string, r containerRunner) *WebWasmContainerBackend {
	if imageRef == "" {
		imageRef = "ghcr.io/vasic-digital/crossbuild-web-wasm:jdk21"
	}
	if expectedOutputs == nil {
		expectedOutputs = defaultWasmOutputs()
	}
	return &WebWasmContainerBackend{imageRef: imageRef, expectedOutputs: expectedOutputs, runner: r}
}

func (w *WebWasmContainerBackend) Name() string { return "web-wasm-container" }

func (w *WebWasmContainerBackend) Capabilities() Capabilities {
	return Capabilities{
		SupportsTargets: []Target{
			{OS: "js", Arch: "wasm"},
		},
		// Wasm-in-JVM-container works on every host OS where rootless
		// podman or docker is present. No host restriction.
		RequiresHostOS:      nil,
		IsolatesEnvironment: true,
		ArtifactNotes: "WebAssembly + JS browser distribution bundle via KGP wasmJsBrowserDistribution " +
			"(requires KGP > 2.1.x for production build — see #wasmjs-production-distribution-gap)",
	}
}

func (w *WebWasmContainerBackend) Build(ctx context.Context, req BuildRequest) BuildResult {
	start := time.Now()
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 45 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := validateRequest(req); err != nil {
		return BuildResult{Target: req.Target, BackendName: w.Name(), Error: err, Duration: time.Since(start)}
	}

	if !w.runner.ImageExists(ctx, w.imageRef) {
		return BuildResult{
			Target:      req.Target,
			BackendName: w.Name(),
			Duration:    time.Since(start),
			Error: fmt.Errorf(
				"crossbuild-web-wasm image %q not present on host; "+
					"provision per docs/crossbuild/web-wasm-image-provisioning.md "+
					"(SKIP-OK: #crossbuild-web-wasm-image-provisioning if intentional)",
				w.imageRef),
		}
	}

	var stdout, stderr bytes.Buffer
	exitCode, err := w.runner.Run(ctx, containerRunSpec{
		Image:       w.imageRef,
		MountSource: req.SourceDir,
		MountTarget: "/work/src",
		WorkDir:     "/work/src",
		Command:     req.BuildCommand,
		Environment: req.Environment,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})

	result := BuildResult{
		Target:      req.Target,
		BackendName: w.Name(),
		StdoutTail:  tailString(stdout.String(), 4096),
		StderrTail:  tailString(stderr.String(), 4096),
		Duration:    time.Since(start),
	}
	if err != nil {
		result.Error = fmt.Errorf("web-wasm container build failed (exit=%d): %w", exitCode, err)
		return result
	}
	if exitCode != 0 {
		result.Error = fmt.Errorf("web-wasm container build exited %d", exitCode)
		return result
	}

	// Verify the primary output artefact via OutputSubpath (index.html).
	produced := filepath.Join(req.SourceDir, req.OutputSubpath)
	stat, err := os.Stat(produced)
	if err != nil {
		result.Error = fmt.Errorf(
			"web-wasm build succeeded (exit=0) but primary artifact missing at %s: %w\n"+
				"This is a known symptom of #wasmjs-production-distribution-gap — "+
				"wasmJsBrowserDistribution exits 0 but omits output on KGP <= 2.0.x. "+
				"Upgrade to KGP > 2.1.x to resolve.",
			produced, err)
		return result
	}
	if stat.Size() == 0 {
		result.Error = fmt.Errorf(
			"web-wasm build produced zero-byte primary artifact at %s (anti-bluff: bluff detected; "+
				"suspect #wasmjs-production-distribution-gap)",
			produced)
		return result
	}

	// Copy primary artifact to HostOutputDir.
	if err := os.MkdirAll(req.HostOutputDir, 0o755); err != nil {
		result.Error = fmt.Errorf("creating HostOutputDir %s: %w", req.HostOutputDir, err)
		return result
	}
	dst := filepath.Join(req.HostOutputDir, filepath.Base(produced))
	if err := copyFile(produced, dst); err != nil {
		result.Error = fmt.Errorf("copying primary artifact to HostOutputDir: %w", err)
		return result
	}
	result.ArtifactPath = dst
	result.ArtifactSize = stat.Size()

	// CONST-039 evidence: verify all expected Wasm bundle outputs.
	// A successful wasmJsBrowserDistribution MUST produce index.html +
	// at least one .wasm file + at least one .js file. Absence of any
	// is a bluff (build exited 0 but didn't produce the deliverable).
	outputDir := filepath.Dir(produced)
	missingEvidence := verifyWasmOutputs(outputDir, w.expectedOutputs)
	if len(missingEvidence) > 0 {
		result.Error = fmt.Errorf(
			"web-wasm build CONST-039 evidence check failed: expected output files missing "+
				"under %s: %v\n"+
				"If running KGP <= 2.0.x, see #wasmjs-production-distribution-gap",
			outputDir, missingEvidence)
		return result
	}

	return result
}

// verifyWasmOutputs checks that every pattern in expected exists under
// outputDir as a non-empty file. Returns names of patterns with no match.
func verifyWasmOutputs(outputDir string, expected []string) []string {
	var missing []string
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return expected // can't read dir — all missing
	}
	for _, pat := range expected {
		found := false
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			matched, _ := filepath.Match(pat, e.Name())
			if matched {
				info, err := e.Info()
				if err == nil && info.Size() > 0 {
					found = true
					break
				}
			}
		}
		if !found {
			missing = append(missing, pat)
		}
	}
	return missing
}

// defaultWasmOutputs returns the file-name glob patterns that
// wasmJsBrowserDistribution is expected to produce. The patterns are
// intentionally generic (no consumer-project name embedded) so this
// backend works for any Kotlin/Wasm consumer project.
func defaultWasmOutputs() []string {
	return []string{
		"index.html",
		"*.wasm",
		"*.js",
	}
}
