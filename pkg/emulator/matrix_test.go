package emulator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmulator records every method invocation and returns
// caller-supplied canned outcomes. Anti-bluff Third Law: the fake's
// behaviour mirrors the production AndroidEmulator's contract — Boot
// returns a BootResult, WaitForBoot returns a duration or an error,
// etc. A test fake that DROPPED a contract method (e.g. Install) would
// be a bluff fake; this one implements every method the contract
// declares.
type fakeEmulator struct {
	bootResults     []BootResult
	waitDurations   []time.Duration
	waitErrors      []error
	installErrors   []error
	runOutputs      []string
	runPassed       []bool
	runErrors       []error
	teardownErrors  []error
	bootCallCount   int
	waitCallCount   int
	installCount    int
	runCount        int
	teardownCount   int
}

func (f *fakeEmulator) Boot(_ context.Context, avd AVD, _ bool) (BootResult, error) {
	idx := f.bootCallCount
	f.bootCallCount++
	r := BootResult{AVD: avd, Started: true, BootCompleted: true,
		ConsolePort: 5554, ADBPort: 5555}
	if idx < len(f.bootResults) {
		r = f.bootResults[idx]
		r.AVD = avd
	}
	if r.Error != nil {
		return r, r.Error
	}
	return r, nil
}

func (f *fakeEmulator) WaitForBoot(_ context.Context, _ int, _ time.Duration) (time.Duration, error) {
	idx := f.waitCallCount
	f.waitCallCount++
	if idx < len(f.waitErrors) {
		// Even on error the contract returns the elapsed time (matters
		// for the "boot timed out at 5m12s" diagnostic). Use the
		// caller-supplied waitDurations[idx] if present.
		if idx < len(f.waitDurations) {
			return f.waitDurations[idx], f.waitErrors[idx]
		}
		return 0, f.waitErrors[idx]
	}
	if idx < len(f.waitDurations) {
		return f.waitDurations[idx], nil
	}
	return 100 * time.Millisecond, nil
}

func (f *fakeEmulator) Install(_ context.Context, _ int, _ string) error {
	idx := f.installCount
	f.installCount++
	if idx < len(f.installErrors) {
		return f.installErrors[idx]
	}
	return nil
}

func (f *fakeEmulator) RunInstrumentation(
	_ context.Context, _ int, _ string, _ time.Duration,
) (string, bool, error) {
	idx := f.runCount
	f.runCount++
	out := ""
	if idx < len(f.runOutputs) {
		out = f.runOutputs[idx]
	}
	passed := false
	if idx < len(f.runPassed) {
		passed = f.runPassed[idx]
	}
	var err error
	if idx < len(f.runErrors) {
		err = f.runErrors[idx]
	}
	return out, passed, err
}

func (f *fakeEmulator) Teardown(_ context.Context, _ int) error {
	idx := f.teardownCount
	f.teardownCount++
	if idx < len(f.teardownErrors) {
		return f.teardownErrors[idx]
	}
	return nil
}

func writeFakeAPK(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fake-*.apk")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// TestAndroidMatrixRunner_RejectsEmptyAVDList pins the precondition.
// Falsifiability: remove the empty-check → matrix continues with no
// boots, writes an empty attestation, and the operator's tag.sh might
// accept it (clause 6.I clause 7 violation).
func TestAndroidMatrixRunner_RejectsEmptyAVDList(t *testing.T) {
	runner := NewAndroidMatrixRunner(&fakeEmulator{})
	_, err := runner.RunMatrix(context.Background(), MatrixConfig{
		APKPath:     writeFakeAPK(t),
		TestClass:   "lava.app.challenges.Challenge01AppLaunchAndTrackerSelectionTest",
		EvidenceDir: t.TempDir(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AVDs is empty")
}

// TestAndroidMatrixRunner_RejectsEmptyAPKPath pins the precondition.
func TestAndroidMatrixRunner_RejectsEmptyAPKPath(t *testing.T) {
	runner := NewAndroidMatrixRunner(&fakeEmulator{})
	_, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        []AVD{{Name: "A"}},
		TestClass:   "T",
		EvidenceDir: t.TempDir(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APKPath is empty")
}

// TestAndroidMatrixRunner_RejectsEmptyTestClass pins the precondition.
func TestAndroidMatrixRunner_RejectsEmptyTestClass(t *testing.T) {
	runner := NewAndroidMatrixRunner(&fakeEmulator{})
	_, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        []AVD{{Name: "A"}},
		APKPath:     writeFakeAPK(t),
		EvidenceDir: t.TempDir(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TestClass is empty")
}

// TestAndroidMatrixRunner_RejectsEmptyEvidenceDir pins the precondition.
func TestAndroidMatrixRunner_RejectsEmptyEvidenceDir(t *testing.T) {
	runner := NewAndroidMatrixRunner(&fakeEmulator{})
	_, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:      []AVD{{Name: "A"}},
		APKPath:   writeFakeAPK(t),
		TestClass: "T",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EvidenceDir is empty")
}

// TestAndroidMatrixRunner_AllAVDsPass_ReportsAllPassed pins the
// happy-path. Falsifiability: drop the AllPassed iteration over
// f.Boots → the function would always return true based on Tests
// alone, and a boot-fail without a test-fail would slip the gate.
func TestAndroidMatrixRunner_AllAVDsPass_ReportsAllPassed(t *testing.T) {
	fake := &fakeEmulator{
		runPassed: []bool{true, true, true},
		runOutputs: []string{
			"BUILD SUCCESSFUL", "BUILD SUCCESSFUL", "BUILD SUCCESSFUL",
		},
	}
	runner := NewAndroidMatrixRunner(fake)
	avds := []AVD{
		{Name: "API28", APILevel: 28, FormFactor: "phone"},
		{Name: "API30", APILevel: 30, FormFactor: "phone"},
		{Name: "API34", APILevel: 34, FormFactor: "phone"},
	}
	evidenceDir := t.TempDir()
	apk := writeFakeAPK(t)

	result, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        avds,
		APKPath:     apk,
		TestClass:   "lava.app.challenges.Challenge01AppLaunchAndTrackerSelectionTest",
		EvidenceDir: evidenceDir,
		ColdBoot:    true,
	})
	require.NoError(t, err)
	assert.True(t, result.AllPassed())
	assert.Len(t, result.Boots, 3)
	assert.Len(t, result.Tests, 3)
	assert.Equal(t, 3, fake.bootCallCount)
	assert.Equal(t, 3, fake.runCount)
	assert.Equal(t, 3, fake.teardownCount,
		"every booted AVD MUST be torn down (clause 6.B)")

	// Attestation file must exist + be valid JSON with 3 rows.
	attestationPath := filepath.Join(evidenceDir, "real-device-verification.json")
	assert.Equal(t, attestationPath, result.AttestationFile)
	bytes, err := os.ReadFile(attestationPath)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(bytes, &doc))
	rows, ok := doc["rows"].([]any)
	require.True(t, ok)
	assert.Len(t, rows, 3)
	assert.Equal(t, true, doc["all_passed"])

	// Group A-prime: verify per-AVD gradle.log is written by RunMatrix.
	// Falsifiability: skip the os.WriteFile call in matrix.go's RunMatrix
	// → these assertions fail because the files don't exist.
	for _, avd := range avds {
		logPath := filepath.Join(evidenceDir, avd.Name, "gradle.log")
		require.FileExists(t, logPath, "gradle.log must be written for AVD %s", avd.Name)
		content, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Equal(t, "BUILD SUCCESSFUL", string(content),
			"gradle.log for %s must contain the captured runOutputs[i]", avd.Name)
	}
}

// TestAndroidMatrixRunner_BootFailure_RecordsRowAndContinues pins the
// continue-on-failure behaviour required by clause 6.I clause 4
// ("missing rows are missing evidence — every AVD MUST get a row").
// Falsifiability: change `continue` to `return result, err` after a
// boot failure → only the failing AVD's row is recorded; later AVDs
// have no row.
func TestAndroidMatrixRunner_BootFailure_RecordsRowAndContinues(t *testing.T) {
	fake := &fakeEmulator{
		bootResults: []BootResult{
			{Started: false, Error: errors.New("kvm denied")},
			{Started: true, BootCompleted: true, ADBPort: 5557},
		},
		runPassed:  []bool{false, true}, // only the second AVD's run is meaningful
		runOutputs: []string{"", "BUILD SUCCESSFUL"},
	}
	runner := NewAndroidMatrixRunner(fake)
	evidenceDir := t.TempDir()

	result, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs: []AVD{
			{Name: "BrokenAVD"},
			{Name: "GoodAVD"},
		},
		APKPath:     writeFakeAPK(t),
		TestClass:   "T",
		EvidenceDir: evidenceDir,
	})
	require.NoError(t, err)
	assert.False(t, result.AllPassed(),
		"AllPassed MUST be false when any boot failed")
	assert.Len(t, result.Boots, 2,
		"both AVDs MUST have a Boot row even when the first failed")
	assert.Len(t, result.Tests, 2,
		"both AVDs MUST have a Test row even when the first boot failed")
}

// TestAndroidMatrixRunner_BootSuccess_FlipsBootCompleted pins the
// happy-path Boot-row state. Without this, the Boot rows in the
// attestation file would always report BootCompleted=false (zero
// value) even when the AVD booted successfully — which would lie to
// scripts/tag.sh's gating check.
//
// Falsifiability:
//   Mutation: in matrix.go, drop the `boot.BootCompleted = true` line
//             added after WaitForBoot succeeds.
//   Observed-Failure: this test asserts BootCompleted=true on the
//             happy-path Boot row; the assertion fires.
//   Reverted: yes (see git log).
//
// Forensic anchor: the 2026-05-04 first-matrix-smoke-run produced
// `all_passed=false` even though the test passed, because the Boot
// row had BootCompleted=false. This test prevents that regression.
func TestAndroidMatrixRunner_BootSuccess_FlipsBootCompleted(t *testing.T) {
	fake := &fakeEmulator{
		runPassed:  []bool{true},
		runOutputs: []string{"BUILD SUCCESSFUL"},
	}
	runner := NewAndroidMatrixRunner(fake)
	result, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        []AVD{{Name: "A"}},
		APKPath:     writeFakeAPK(t),
		TestClass:   "T",
		EvidenceDir: t.TempDir(),
	})
	require.NoError(t, err)
	require.Len(t, result.Boots, 1)
	assert.True(t, result.Boots[0].BootCompleted,
		"happy-path Boot row MUST have BootCompleted=true so AllPassed() reflects reality")
	assert.True(t, result.AllPassed())
}

// TestAndroidMatrixRunner_TestFailure_PropagatesToAttestation pins the
// failing-test path. Falsifiability: in writeAttestation, hard-code
// all_passed=true → this test fails because the attestation file
// would lie about the run.
func TestAndroidMatrixRunner_TestFailure_PropagatesToAttestation(t *testing.T) {
	fake := &fakeEmulator{
		runPassed:  []bool{false},
		runOutputs: []string{"BUILD FAILED — assertion fired"},
		runErrors:  []error{errors.New("test assertions failed")},
	}
	runner := NewAndroidMatrixRunner(fake)
	evidenceDir := t.TempDir()

	result, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        []AVD{{Name: "A"}},
		APKPath:     writeFakeAPK(t),
		TestClass:   "T",
		EvidenceDir: evidenceDir,
	})
	require.NoError(t, err)
	assert.False(t, result.AllPassed())
	bytes, err := os.ReadFile(result.AttestationFile)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(bytes, &doc))
	assert.Equal(t, false, doc["all_passed"])
	rows := doc["rows"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	assert.Equal(t, false, row["test_passed"])
	assert.NotEmpty(t, row["test_error"])
}

// TestAndroidMatrixRunner_BootDuration_IncludesWaitForBootElapsed pins
// the clause 6.I clause 6 cold-boot-only audit fix (2026-05-04 evening):
// the user-visible "boot_seconds" in the attestation file MUST be the
// total time from emulator launch to sys.boot_completed=1 — i.e. the
// sum of the launch-command duration and the WaitForBoot poll duration.
//
// Before this fix, matrix.go reported only the launch-command duration
// (microseconds in practice — the emulator binary returns immediately
// after backgrounding itself), making `boot_seconds` near-zero and the
// clause 6.I clause 6 audit ("cold-boot only for the gate run") vacuous
// because the field could not distinguish a true 5-minute cold boot
// from a snapshot reload.
//
// Falsifiability rehearsal:
//   Mutation: in matrix.go, remove the line `boot.BootDuration += waitDuration`.
//   Observed-Failure: this test fails — boot_seconds in the attestation
//             is 0.05 (the launch-command duration) instead of the 7.05
//             total (5s wait + 0.05 launch).
//   Reverted: yes — post-revert the test passes.
func TestAndroidMatrixRunner_BootDuration_IncludesWaitForBootElapsed(t *testing.T) {
	fake := &fakeEmulator{
		bootResults: []BootResult{
			{
				Started:      true,
				BootDuration: 50 * time.Millisecond,
				ConsolePort:  5554,
				ADBPort:      5555,
			},
		},
		waitDurations: []time.Duration{7 * time.Second},
		runPassed:     []bool{true},
		runOutputs:    []string{"BUILD SUCCESSFUL"},
	}
	runner := NewAndroidMatrixRunner(fake)
	evidenceDir := t.TempDir()
	result, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        []AVD{{Name: "A", APILevel: 34, FormFactor: "phone"}},
		APKPath:     writeFakeAPK(t),
		TestClass:   "T",
		EvidenceDir: evidenceDir,
	})
	require.NoError(t, err)
	require.Len(t, result.Boots, 1)

	// PRIMARY assertion: boot duration on the result equals launch + wait
	expected := 50*time.Millisecond + 7*time.Second
	assert.Equal(t, expected, result.Boots[0].BootDuration,
		"boot_seconds MUST capture launch-command duration PLUS WaitForBoot elapsed")

	// SECONDARY assertion: the on-disk attestation reflects the same.
	bytes, err := os.ReadFile(result.AttestationFile)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(bytes, &doc))
	rows := doc["rows"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	bootSec := row["boot_seconds"].(float64)
	assert.InDelta(t, 7.05, bootSec, 0.01,
		"on-disk boot_seconds MUST reflect the user-visible cold-boot wall-clock; got %v", bootSec)
}

// TestAndroidMatrixRunner_GradleLogWriteFailure_DoesNotFailRun confirms
// the gradle.log persistence is best-effort: if the write to
// EvidenceDir/<avd>/gradle.log fails (read-only filesystem, permission
// denied, disk full, etc.), the matrix run MUST still report
// all_passed=true based on the actual test outcomes.
//
// Per the Group A-prime spec Section I — the gradle.log persistence is
// observability, not a gate. The matrix runner's pass/fail signal comes
// from the test outcomes themselves, not from whether we could also
// persist the stdout for post-mortem.
//
// Falsifiability: the best-effort guard is structural non-falsifiable in
// the current implementation — the inner write code only logs to stderr
// on failure and never returns an error or fails the matrix. Removing the
// OUTER `if mkErr := os.MkdirAll(avdDir, 0o755); mkErr == nil` guard
// would mean the inner code runs even when MkdirAll failed; the inner
// code's self-handling absorbs the cascade. A future regression that DID
// propagate write errors up to RunMatrix's return value WOULD be caught
// by this test's `require.NoError(t, err)`. A future regression that
// changed best-effort semantics to fail-loud (i.e., returning an error
// from RunMatrix on write failure) would also be caught by this test.
// Both classes of regression are guarded. The structural limit is
// documented — not a bluff — because the test asserts the positive
// contract callers depend on.
func TestAndroidMatrixRunner_GradleLogWriteFailure_DoesNotFailRun(t *testing.T) {
	fake := &fakeEmulator{
		runPassed:  []bool{true},
		runOutputs: []string{"BUILD SUCCESSFUL"},
	}
	runner := NewAndroidMatrixRunner(fake)

	// Pre-create EvidenceDir as a read-only directory. RunMatrix's
	// top-level os.MkdirAll(config.EvidenceDir, ...) is a no-op on an
	// existing directory (regardless of its permissions), so the matrix
	// continues. The per-AVD os.MkdirAll inside the gradle.log block
	// tries to create a subdirectory under the read-only parent and
	// fails with EACCES — exercising the best-effort failure path
	// without requiring filesystem-level chmod gymnastics on the actual
	// gradle.log file.
	//
	// Note: the attestation write (writeAttestation) also fails under a
	// read-only EvidenceDir, but that path is guarded by
	// `if err == nil { result.AttestationFile = ... }` — the error is
	// silently dropped and RunMatrix returns nil. So we assert neither
	// the attestation file path nor its contents — only AllPassed().
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "ro-evidence")
	require.NoError(t, os.Mkdir(evidenceDir, 0o755))
	require.NoError(t, os.Chmod(evidenceDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(evidenceDir, 0o755) }) // restore so t.TempDir() cleanup can delete it

	result, err := runner.RunMatrix(context.Background(), MatrixConfig{
		AVDs:        []AVD{{Name: "API34", APILevel: 34, FormFactor: "phone"}},
		APKPath:     writeFakeAPK(t),
		TestClass:   "T",
		EvidenceDir: evidenceDir,
	})

	// PRIMARY assertion: RunMatrix MUST NOT return an error just because
	// gradle.log persistence failed. Write errors are best-effort.
	require.NoError(t, err,
		"matrix MUST NOT fail just because gradle.log persistence failed")

	// PRIMARY assertion: the pass/fail signal comes from the actual test
	// outcomes, not from whether the on-disk log could be written.
	assert.True(t, result.AllPassed(),
		"matrix MUST report all_passed=true based on actual test outcomes, "+
			"NOT on gradle.log write success")
}
