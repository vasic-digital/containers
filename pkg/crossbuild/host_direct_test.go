package crossbuild

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner implements processRunner for orchestration tests. It
// records what was asked + lets the test control exit code + which
// file the build "produces".
type fakeRunner struct {
	expectedDir     string
	expectedCommand string
	exitCode        int
	errOnRun        error
	stdout          string
	stderr          string

	// producesPath: if non-empty, fakeRunner creates a file at this
	// absolute path with producesContent before returning. Simulates
	// the build dropping its artifact in the right place.
	producesPath    string
	producesContent []byte

	gotDir     string
	gotCommand string
	gotEnv     map[string]string
}

func (f *fakeRunner) Run(ctx context.Context, dir, command string, env map[string]string,
	stdout, stderr *bytes.Buffer) (int, error) {
	f.gotDir = dir
	f.gotCommand = command
	f.gotEnv = env
	stdout.WriteString(f.stdout)
	stderr.WriteString(f.stderr)
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

// TestHostDirect_HappyPath produces a real artifact (8 bytes of test
// content) and verifies the orchestrator copies it to HostOutputDir
// with the right size. Positive runtime evidence per CONST-035.
func TestHostDirect_HappyPath(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "build/out/artifact.bin"

	runner := &fakeRunner{
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: []byte("ARTIFACT"),
		stdout:          "compile OK\n",
	}
	hd := newHostDirectBackendWithRunner(runner)

	result := hd.Build(context.Background(), BuildRequest{
		Target:        Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew :desktopApp:packageReleaseDeb",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
		Timeout:       1 * time.Minute,
	})

	require.NoError(t, result.Error,
		"happy-path build must succeed when runner exits 0 + artifact exists")
	assert.Equal(t, "host-direct", result.BackendName)
	assert.Equal(t, int64(len("ARTIFACT")), result.ArtifactSize)
	assert.Equal(t, filepath.Join(outDir, "artifact.bin"), result.ArtifactPath)

	// Positive runtime evidence: the artifact lives where promised.
	stat, err := os.Stat(result.ArtifactPath)
	require.NoError(t, err)
	assert.Equal(t, int64(len("ARTIFACT")), stat.Size())

	// Runner observed the right inputs.
	assert.Equal(t, srcDir, runner.gotDir)
	assert.Contains(t, runner.gotCommand, "packageReleaseDeb")
}

// TestHostDirect_RejectsZeroByteArtifact enforces the anti-bluff
// invariant: a BUILD SUCCESSFUL that produces a zero-byte artifact
// MUST be treated as failure, not silent success.
func TestHostDirect_RejectsZeroByteArtifact(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "build/empty.bin"

	runner := &fakeRunner{
		exitCode:        0,
		producesPath:    filepath.Join(srcDir, subpath),
		producesContent: []byte{}, // 0 bytes — bluff territory
	}
	hd := newHostDirectBackendWithRunner(runner)

	result := hd.Build(context.Background(), BuildRequest{
		Target:        Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		SourceDir:     srcDir,
		BuildCommand:  "./gradlew assemble",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "zero-byte artifact",
		"zero-byte artifact must be flagged as bluff")
	assert.Empty(t, result.ArtifactPath, "no path returned on failure")
}

// TestHostDirect_RejectsMissingArtifact enforces the anti-bluff
// invariant: BUILD SUCCESSFUL that produces NO artifact MUST fail.
func TestHostDirect_RejectsMissingArtifact(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()
	subpath := "build/should-exist-but-doesnt.bin"

	runner := &fakeRunner{
		exitCode:     0,
		producesPath: "", // runner doesn't create the file
	}
	hd := newHostDirectBackendWithRunner(runner)

	result := hd.Build(context.Background(), BuildRequest{
		Target:        Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		SourceDir:     srcDir,
		BuildCommand:  "true",
		OutputSubpath: subpath,
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "artifact missing")
	assert.Contains(t, result.Error.Error(), "bluff",
		"error message must call out the anti-bluff classification")
}

// TestHostDirect_PropagatesNonZeroExit covers the path where the
// build command itself fails (exit > 0). Result.Error must be set
// + StderrTail captured.
func TestHostDirect_PropagatesNonZeroExit(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	runner := &fakeRunner{
		exitCode: 2,
		stderr:   "compile error: undefined symbol\n",
	}
	hd := newHostDirectBackendWithRunner(runner)

	result := hd.Build(context.Background(), BuildRequest{
		Target:        Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		SourceDir:     srcDir,
		BuildCommand:  "false",
		OutputSubpath: "out.bin",
		HostOutputDir: outDir,
	})

	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "exited 2")
	assert.Contains(t, result.StderrTail, "compile error")
}

// TestHostDirect_ValidatesRequest exercises every required-field
// check. Saves consumers from confusing "no such file" errors deep
// in the stack.
func TestHostDirect_ValidatesRequest(t *testing.T) {
	hd := NewHostDirectBackend()
	tests := []struct {
		name string
		req  BuildRequest
		want string
	}{
		{"missing SourceDir", BuildRequest{Target: Target{OS: "linux", Arch: "amd64"}}, "SourceDir is required"},
		{"relative SourceDir", BuildRequest{
			Target:        Target{OS: "linux", Arch: "amd64"},
			SourceDir:     "relative/path",
			BuildCommand:  "true",
			OutputSubpath: "x",
			HostOutputDir: "/tmp",
		}, "must be absolute"},
		{"missing BuildCommand", BuildRequest{
			Target:        Target{OS: "linux", Arch: "amd64"},
			SourceDir:     "/tmp",
			OutputSubpath: "x",
			HostOutputDir: "/tmp",
		}, "BuildCommand is required"},
		{"missing OutputSubpath", BuildRequest{
			Target:        Target{OS: "linux", Arch: "amd64"},
			SourceDir:     "/tmp",
			BuildCommand:  "true",
			HostOutputDir: "/tmp",
		}, "OutputSubpath is required"},
		{"missing HostOutputDir", BuildRequest{
			Target:        Target{OS: "linux", Arch: "amd64"},
			SourceDir:     "/tmp",
			BuildCommand:  "true",
			OutputSubpath: "x",
		}, "HostOutputDir is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hd.Build(context.Background(), tc.req)
			require.Error(t, result.Error)
			assert.Contains(t, result.Error.Error(), tc.want)
		})
	}
}

// TestTailString_ReturnsLastNChars locks the diagnostics-tail helper.
func TestTailString_ReturnsLastNChars(t *testing.T) {
	assert.Equal(t, "abc", tailString("abc", 10))
	assert.Equal(t, "…cde", tailString("abcde", 3))
	assert.Equal(t, "", tailString("", 10))
}
