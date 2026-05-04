package emulator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExecutor records every (name, args) invocation and returns
// caller-supplied canned output. Anti-bluff posture (clauses 6.J/6.L
// inherited from Containers' parent constitution): the fake is
// behaviorally equivalent to os/exec for the assertions we care about
// — it captures the same arguments the real executor would receive,
// and it returns bytes the real executor would also return for the
// scripted scenario. A test that asserts on `fake.calls` is asserting
// on the same observable behaviour that a real-shell smoke test would
// see in the host's process accounting.
type fakeExecutor struct {
	calls   []fakeCall
	scripts map[string]fakeScript
}

type fakeCall struct {
	Name string
	Args []string
}

type fakeScript struct {
	Out []byte
	Err error
}

func (f *fakeExecutor) Execute(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, fakeCall{Name: name, Args: append([]string{}, args...)})
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}
	if s, ok := f.scripts[key]; ok {
		return s.Out, s.Err
	}
	if s, ok := f.scripts[name]; ok {
		return s.Out, s.Err
	}
	return nil, nil
}

// Start mirrors the production Start contract — records the call and
// returns the script's err (if any). The production Start launches a
// long-running process and returns nil immediately on success; the
// fake matches that for short tests.
func (f *fakeExecutor) Start(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, fakeCall{Name: name, Args: append([]string{}, args...)})
	if s, ok := f.scripts[name]; ok {
		return s.Err
	}
	return nil
}

// TestAndroidEmulator_Boot_PassesExpectedFlagsToEmulatorBinary verifies
// the production code path passes the documented flags to the SDK's
// emulator binary. Falsifiability:
//
//   Mutation: in android.go Boot, replace `-no-snapshot` with
//             `-snapshot-load`. Re-run this test.
//   Observed-Failure: assertion on the args slice fires because the
//             flag string changes.
//   Reverted: yes (see git log).
func TestAndroidEmulator_Boot_PassesExpectedFlagsToEmulatorBinary(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)

	avd := AVD{Name: "Pixel_9a", APILevel: 36, FormFactor: "phone"}
	result, err := emu.Boot(context.Background(), avd, true)

	require.NoError(t, err)
	assert.True(t, result.Started)
	require.Len(t, exec.calls, 1)
	call := exec.calls[0]
	assert.Equal(t, "/sdk/emulator/emulator", call.Name)
	assert.Contains(t, call.Args, "-avd")
	assert.Contains(t, call.Args, "Pixel_9a")
	assert.Contains(t, call.Args, "-no-window")
	assert.Contains(t, call.Args, "-no-audio")
	assert.Contains(t, call.Args, "-no-snapshot",
		"clause 6.I clause 6 — cold-boot MUST set -no-snapshot")
}

// TestAndroidEmulator_Boot_WithoutColdBoot_OmitsNoSnapshotFlag pins
// the ColdBoot=false branch. Falsifiability:
//
//   Mutation: in Boot, always append `-no-snapshot` regardless of
//             coldBoot. Re-run.
//   Observed-Failure: this test fails because -no-snapshot would be
//             present even though ColdBoot was false.
func TestAndroidEmulator_Boot_WithoutColdBoot_OmitsNoSnapshotFlag(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	_, err := emu.Boot(context.Background(), AVD{Name: "X"}, false)
	require.NoError(t, err)
	require.Len(t, exec.calls, 1)
	for _, a := range exec.calls[0].Args {
		assert.NotEqual(t, "-no-snapshot", a)
	}
}

// TestAndroidEmulator_Boot_PropagatesExecutorError pins the error path.
func TestAndroidEmulator_Boot_PropagatesExecutorError(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{
		"/sdk/emulator/emulator": {Err: errors.New("emulator not found")},
	}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	result, err := emu.Boot(context.Background(), AVD{Name: "X"}, true)
	require.Error(t, err)
	assert.False(t, result.Started)
	assert.NotNil(t, result.Error)
}

// TestAndroidEmulator_WaitForBoot_ReturnsImmediatelyWhenBootCompleted
// pins the happy path.
func TestAndroidEmulator_WaitForBoot_ReturnsImmediatelyWhenBootCompleted(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{
		"/sdk/platform-tools/adb -s localhost:5555 shell getprop sys.boot_completed": {
			Out: []byte("1\n"),
		},
	}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	dur, err := emu.WaitForBoot(context.Background(), 5555, 30*time.Second)
	require.NoError(t, err)
	assert.True(t, dur < 30*time.Second)
}

// TestAndroidEmulator_WaitForBoot_TimesOutWhenNeverBoots pins the
// timeout path. Uses a very short timeout to keep the test fast.
func TestAndroidEmulator_WaitForBoot_TimesOutWhenNeverBoots(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{
		"/sdk/platform-tools/adb -s localhost:5555 shell getprop sys.boot_completed": {
			Out: []byte("0\n"),
		},
	}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	_, err := emu.WaitForBoot(context.Background(), 5555, 1*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boot not completed")
}

// TestAndroidEmulator_Install_RejectsMissingAPK pins the precondition
// check. Falsifiability: drop the os.Stat check → this test passes
// the missing-apk path through to adb (where it would fail late and
// confusingly).
func TestAndroidEmulator_Install_RejectsMissingAPK(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	err := emu.Install(context.Background(), 5555, "/nonexistent/path.apk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apk not found")
	// adb MUST NOT be invoked when the apk is missing — early-return
	// preserves the user-visible error context.
	assert.Empty(t, exec.calls,
		"adb invocation MUST NOT happen when apk file is missing")
}

// TestAndroidEmulator_Teardown_InvokesAdbEmuKill pins the teardown
// path. Falsifiability: change "kill" to anything else → this test
// fails because the Args slice no longer contains "kill".
func TestAndroidEmulator_Teardown_InvokesAdbEmuKill(t *testing.T) {
	exec := &fakeExecutor{scripts: map[string]fakeScript{}}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	err := emu.Teardown(context.Background(), 5555)
	require.NoError(t, err)
	require.Len(t, exec.calls, 1)
	c := exec.calls[0]
	assert.Equal(t, "/sdk/platform-tools/adb", c.Name)
	expected := []string{"-s", "localhost:5555", "emu", "kill"}
	assert.Equal(t, expected, c.Args)
}

// TestAndroidEmulator_RunInstrumentation_RejectsEmptyTestClass pins
// the precondition. Anti-bluff: a runner that silently runs all tests
// when testClass is empty would mask a configuration bug.
func TestAndroidEmulator_RunInstrumentation_RejectsEmptyTestClass(t *testing.T) {
	emu := NewAndroidEmulator("/sdk")
	_, _, err := emu.RunInstrumentation(
		context.Background(), 5555, "", 30*time.Second,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "testClass MUST be non-empty")
}

func init() {
	_ = fmt.Sprintf // keep "fmt" usable for future test extensions
}
