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
		return 0, f.waitErrors[idx]
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
