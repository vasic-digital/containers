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
	// sequencedScripts returns successive outputs for repeated calls
	// to the same command key, simulating real adb output that changes
	// between invocations (e.g. `adb devices` when a new emulator
	// appears mid-test). The last entry repeats after exhaustion.
	sequencedScripts map[string][]fakeScript
	seqIdx           map[string]int
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
	// Sequenced scripts take precedence — each call returns the next
	// entry in the sequence (last entry repeats forever).
	if seq, ok := f.sequencedScripts[key]; ok && len(seq) > 0 {
		if f.seqIdx == nil {
			f.seqIdx = map[string]int{}
		}
		idx := f.seqIdx[key]
		if idx >= len(seq) {
			idx = len(seq) - 1
		}
		f.seqIdx[key] = f.seqIdx[key] + 1
		return seq[idx].Out, seq[idx].Err
	}
	if seq, ok := f.sequencedScripts[name]; ok && len(seq) > 0 {
		if f.seqIdx == nil {
			f.seqIdx = map[string]int{}
		}
		idx := f.seqIdx[name]
		if idx >= len(seq) {
			idx = len(seq) - 1
		}
		f.seqIdx[name] = f.seqIdx[name] + 1
		return seq[idx].Out, seq[idx].Err
	}
	if s, ok := f.scripts[key]; ok {
		return s.Out, s.Err
	}
	if s, ok := f.scripts[name]; ok {
		return s.Out, s.Err
	}
	return nil, nil
}

// adbDevicesScript is a fakeExecutor convenience that returns the
// supplied serials as `adb devices` output. Used by tests that need to
// drive the Boot() port-discovery loop. Pass an EMPTY slice for the
// "no emulators yet" pre-snapshot, then the actual list for the post-
// launch state — the sequenced script will hand them back in order.
func adbDevicesScript(serials ...int) fakeScript {
	var sb strings.Builder
	sb.WriteString("List of devices attached\n")
	for _, s := range serials {
		fmt.Fprintf(&sb, "emulator-%d\tdevice\n", s)
	}
	return fakeScript{Out: []byte(sb.String())}
}

// firstCallMatching returns the index + call where Name == target,
// or panics with a descriptive message. Used by Boot tests after the
// 2026-05-04 architectural fix that introduced pre-snapshot adb-devices
// calls before the emulator-binary launch — calls[0] is no longer the
// launch call.
func firstCallMatching(t *testing.T, calls []fakeCall, target string) fakeCall {
	t.Helper()
	for _, c := range calls {
		if c.Name == target {
			return c
		}
	}
	t.Fatalf("no call to %s found in %d recorded calls", target, len(calls))
	return fakeCall{}
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
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{},
		sequencedScripts: map[string][]fakeScript{
			// Pre-snapshot: empty. Post-launch: emulator-5554 appears.
			// The Boot() port-discovery loop diffs these to find the
			// new serial.
			"/sdk/platform-tools/adb devices": {
				adbDevicesScript(),
				adbDevicesScript(5554),
			},
		},
	}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)

	avd := AVD{Name: "Pixel_9a", APILevel: 36, FormFactor: "phone"}
	result, err := emu.Boot(context.Background(), avd, true)

	require.NoError(t, err)
	assert.True(t, result.Started)
	assert.Equal(t, 5554, result.ConsolePort,
		"Boot MUST capture the discovered console port (5554) from adb devices, not hardcoded")
	assert.Equal(t, 5555, result.ADBPort)
	// Find the launch call — it is no longer calls[0] because the
	// pre-snapshot adb-devices call comes first now.
	call := firstCallMatching(t, exec.calls, "/sdk/emulator/emulator")
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
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{},
		sequencedScripts: map[string][]fakeScript{
			"/sdk/platform-tools/adb devices": {
				adbDevicesScript(),
				adbDevicesScript(5554),
			},
		},
	}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	_, err := emu.Boot(context.Background(), AVD{Name: "X"}, false)
	require.NoError(t, err)
	launch := firstCallMatching(t, exec.calls, "/sdk/emulator/emulator")
	for _, a := range launch.Args {
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
// fails because the kill-call args no longer contain "kill".
func TestAndroidEmulator_Teardown_InvokesAdbEmuKill(t *testing.T) {
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{},
		// 2026-05-05 wait-for-exit: simulate the emulator disappearing
		// from `adb devices` immediately after `adb emu kill`. Without
		// this, the polling loop in Teardown would time out.
		sequencedScripts: map[string][]fakeScript{
			"/sdk/platform-tools/adb devices": {
				{Out: []byte("List of devices attached\n")},
			},
		},
	}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	err := emu.Teardown(context.Background(), 5555)
	require.NoError(t, err)
	// Expect the kill call AND at least one adb-devices poll. The
	// kill must come first so the device is on its way out before we poll.
	require.GreaterOrEqual(t, len(exec.calls), 1)
	first := exec.calls[0]
	assert.Equal(t, "/sdk/platform-tools/adb", first.Name)
	expected := []string{"-s", "localhost:5555", "emu", "kill"}
	assert.Equal(t, expected, first.Args, "first call MUST be the kill")
}

// TestAndroidEmulator_Teardown_WaitsForEmulatorToActuallyExit is the
// regression test for the 2026-05-05 Teardown wait-for-exit fix.
//
// Forensic anchor: `adb -s localhost:5555 emu kill` returns "OK"
// almost immediately, but the qemu-system process can take 10-30s to
// actually exit. The pre-fix Teardown returned immediately, causing
// the matrix runner's next iteration to start while the previous
// emulator was still alive on 5554/5555. The new emulator landed on
// a different port — which the discovery fix (commit 648a4bb) handles
// correctly — but 5 simultaneous emulators caused CPU/RAM contention
// that produced flakes in late iterations.
//
// This test simulates: kill returns OK → first poll shows device
// still alive → second poll shows device gone. Teardown MUST return
// only AFTER the second poll, NOT after the first.
//
// Falsifiability:
//   Mutation: revert Teardown to the pre-fix form (return immediately
//             after kill, no polling).
//   Observed-Failure: this test would fail because the call count
//             would be 1 (kill only), not 2+ (kill + at least one poll).
//   Reverted: yes — post-revert this test passes.
func TestAndroidEmulator_Teardown_WaitsForEmulatorToActuallyExit(t *testing.T) {
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{},
		sequencedScripts: map[string][]fakeScript{
			"/sdk/platform-tools/adb devices": {
				// First poll: emulator still alive (qemu hasn't exited yet)
				{Out: []byte("List of devices attached\nemulator-5554\tdevice\nlocalhost:5555\tdevice\n")},
				// Second poll: emulator gone
				{Out: []byte("List of devices attached\n")},
			},
		},
	}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)
	err := emu.Teardown(context.Background(), 5555)
	require.NoError(t, err)
	// We expect 1 kill call + at least 2 poll calls (one that saw
	// "still alive", one that saw "gone"). Total >= 3.
	devicesCalls := 0
	for _, c := range exec.calls {
		if c.Name == "/sdk/platform-tools/adb" && len(c.Args) == 1 && c.Args[0] == "devices" {
			devicesCalls++
		}
	}
	assert.GreaterOrEqual(t, devicesCalls, 2,
		"Teardown MUST poll adb devices at least twice (once seeing 'alive', once seeing 'gone'); got %d", devicesCalls)
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

// TestAndroidEmulator_Boot_DiscoversNewSerial_WhenPriorEmulatorPersists
// is the regression test for the 2026-05-04 ultrathink-discovered
// architectural bluff in the multi-AVD matrix runner.
//
// Forensic anchor (verbatim from the diagnostic output captured this
// session):
//
//   [matrix-diag] target=localhost:5555 adb-devices=
//     "List of devices attached\nemulator-5554\tdevice\nlocalhost:5555\tdevice"
//     ro.product.model="Android SDK built for x86_64"
//   [matrix-diag] target=localhost:5555 adb-devices=
//     "...emulator-5554\tdevice\nemulator-5556\toffline..."
//     ro.product.model="Android SDK built for x86_64"
//   ... (5 iterations, all targeting localhost:5555, all reporting
//        the same model — every iteration tested the FIRST AVD's
//        emulator while the matrix attestation file claimed each row
//        was a different AVD)
//
// Pre-fix Boot() hardcoded ADBPort=5555 unconditionally. When the
// previous AVD's Teardown left the first emulator alive on 5554/5555
// (which Teardown did fail to kill in this session — 5 orphans
// observed across iterations), the next emulator launched on
// 5556/5557 but the matrix runner kept polling 5555 — getting the
// OLD AVD's "boot complete" signal instantly and running the test
// against the OLD AVD.
//
// This test pins the post-fix behaviour: when `adb devices` already
// shows an emulator on 5554 (a leaked previous emulator), Boot() MUST
// detect that the NEW emulator landed on a different port and return
// THAT port in BootResult.ADBPort.
//
// Falsifiability rehearsal (clause 6.I clause 5 + Sixth Law clause 2):
//
//   Mutation: revert this commit's Boot() to the pre-fix form
//             (hardcoded ConsolePort=5554, ADBPort=5555).
//   Observed-Failure: this test fails with
//             "expected 5557 but got 5555" — the assertion below.
//   Reverted: yes, post-revert this test passes again.
func TestAndroidEmulator_Boot_DiscoversNewSerial_WhenPriorEmulatorPersists(t *testing.T) {
	// Simulate the bug scenario: a prior matrix iteration's emulator
	// is STILL running on 5554 (Teardown failed silently). The new
	// AVD's emulator lands on 5556 because 5554/5555 is busy.
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{},
		sequencedScripts: map[string][]fakeScript{
			"/sdk/platform-tools/adb devices": {
				// Pre-snapshot: prior emulator on 5554 is still alive.
				adbDevicesScript(5554),
				// Post-launch: new emulator on 5556 has appeared.
				adbDevicesScript(5554, 5556),
			},
		},
	}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)

	avd := AVD{Name: "CZ_API30_Phone", APILevel: 30, FormFactor: "phone"}
	result, err := emu.Boot(context.Background(), avd, true)

	require.NoError(t, err)
	require.True(t, result.Started)
	// PRIMARY: the discovered console port MUST be 5556 (the new one),
	// NOT 5554 (the leaked previous one). Falsifiable: revert Boot()'s
	// dynamic discovery → result.ConsolePort would be 5554 (or 5555
	// for the pre-fix hardcode).
	assert.Equal(t, 5556, result.ConsolePort,
		"clause 6.I architectural fix — Boot MUST detect the NEW emulator's "+
			"port via adb-devices diff, NOT return the leaked previous emulator's port")
	assert.Equal(t, 5557, result.ADBPort,
		"ADB port = console port + 1, dynamically computed from discovery")
	// SECONDARY: the AVD metadata is preserved through the discovery
	// step (catches a regression where Boot returns an empty AVD on
	// the discovery path).
	assert.Equal(t, "CZ_API30_Phone", result.AVD.Name)
	assert.Equal(t, 30, result.AVD.APILevel)
}

// TestAndroidEmulator_Boot_FailsWhenNoNewSerialAppears pins the
// failure path of the discovery: if `adb devices` never registers a
// new emulator (kvm denied, zygote crash, etc.), Boot MUST return an
// error rather than silently mis-targeting later calls. Without this
// check, the matrix runner would proceed to Install + RunInstrumentation
// against whatever happened to be on 5555 (the pre-fix bug).
//
// Note: this test passes a custom-shortened context to keep test time
// reasonable. The production timeout is 60s; we simulate it as
// "discovery exhausted" via a deadline-bounded context.
func TestAndroidEmulator_Boot_FailsWhenNoNewSerialAppears(t *testing.T) {
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{},
		sequencedScripts: map[string][]fakeScript{
			"/sdk/platform-tools/adb devices": {
				// Pre and post: same set. No new emulator ever appears.
				adbDevicesScript(5554),
				adbDevicesScript(5554),
			},
		},
	}
	emu := NewAndroidEmulatorWithExecutor("/sdk", exec)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := emu.Boot(ctx, AVD{Name: "X"}, true)

	require.Error(t, err)
	// The wrapped error chain contains either "port discovery cancelled"
	// (ctx deadline beat the 60s timeout) or "no new emulator serial
	// appeared" (60s timeout fired first). Both are correct
	// constitutional behaviour — the runner refused to silently
	// mis-target subsequent calls.
	msg := err.Error()
	assert.True(t,
		strings.Contains(msg, "port discovery") || strings.Contains(msg, "no new emulator serial"),
		"error message MUST mention discovery failure, got: %s", msg)
	assert.True(t, result.Started, "Started=true even on discovery fail — the launch itself succeeded")
	assert.NotNil(t, result.Error)
}
