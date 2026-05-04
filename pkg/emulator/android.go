package emulator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CommandExecutor is the seam through which the AndroidEmulator runs
// host commands. The production impl shells out via os/exec; tests
// substitute a fake that records invocations and returns canned output.
//
// Anti-bluff posture (clause 6.J): the seam exists ONLY for testing.
// Production code uses the real os/exec impl. A test that uses the
// fake to assert "real adb was invoked with these args" is not a bluff
// because it asserts on observable host-shell behaviour, not on internal
// state.
//
// `Execute` is for short-lived synchronous commands (adb, getprop).
// `Start` is for long-running detached processes (the emulator itself
// is a long-lived QEMU-backed process; the matrix runner needs Boot
// to return without blocking on it).
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
	Start(ctx context.Context, name string, args ...string) error
}

type osExecutor struct{}

func (osExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (osExecutor) Start(_ context.Context, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	// Detach: redirect stdio to /dev/null; setsid (POSIX) so the
	// emulator survives the test runner's process group.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	// Release the child's resources so the runner doesn't accumulate
	// zombies. We don't Wait() — the emulator process lives until
	// Teardown sends `adb emu kill`.
	go func() { _ = cmd.Wait() }()
	return nil
}

// NewOSExecutor returns the real os/exec-based executor used by
// production code.
func NewOSExecutor() CommandExecutor { return osExecutor{} }

// AndroidEmulator implements [Emulator] by shelling out to the Android
// SDK's emulator + adb binaries. The runner does NOT itself manage a
// container — clause 6.I says the matrix runs INSIDE a container, and
// the caller-supplied AndroidSdkRoot is the path to the SDK that's
// already mounted into the container (or available on the host for
// development iteration).
//
// Methods follow the Emulator interface; see types.go for the contract.
type AndroidEmulator struct {
	executor       CommandExecutor
	androidSdkRoot string
}

// NewAndroidEmulator constructs an AndroidEmulator that uses the real
// host shell to invoke the SDK binaries.
func NewAndroidEmulator(androidSdkRoot string) *AndroidEmulator {
	return &AndroidEmulator{
		executor:       osExecutor{},
		androidSdkRoot: androidSdkRoot,
	}
}

// NewAndroidEmulatorWithExecutor is the test-injection constructor.
// Production code uses NewAndroidEmulator.
func NewAndroidEmulatorWithExecutor(
	androidSdkRoot string,
	executor CommandExecutor,
) *AndroidEmulator {
	return &AndroidEmulator{executor: executor, androidSdkRoot: androidSdkRoot}
}

func (a *AndroidEmulator) emulatorBinary() string {
	return a.androidSdkRoot + "/emulator/emulator"
}

func (a *AndroidEmulator) adbBinary() string {
	return a.androidSdkRoot + "/platform-tools/adb"
}

// Boot starts the AVD in headless mode. The emulator process runs
// asynchronously; this method returns once the process is detached.
// Use WaitForBoot to wait for Android boot completion.
//
// Per clause 6.I clause 6, coldBoot=true SHOULD be used for any gating
// matrix run — it disables snapshot reload, ensuring reproducibility
// across runs.
func (a *AndroidEmulator) Boot(
	ctx context.Context,
	avd AVD,
	coldBoot bool,
) (BootResult, error) {
	args := []string{
		"-avd", avd.Name,
		"-no-window",
		"-no-audio",
		"-no-boot-anim",
		"-gpu", "swiftshader_indirect",
	}
	if coldBoot {
		args = append(args, "-no-snapshot")
	}

	// We launch the emulator detached. Start (vs Execute) means the
	// underlying process keeps running after this call returns; the
	// caller must invoke Teardown via `adb emu kill` to stop it.
	startedAt := time.Now()
	err := a.executor.Start(ctx, a.emulatorBinary(), args...)
	result := BootResult{
		AVD:          avd,
		Started:      err == nil,
		BootDuration: time.Since(startedAt),
		ConsolePort:  5554, // default first-emulator console port
		ADBPort:      5555, // default first-emulator adb port
		Error:        err,
	}
	if err != nil {
		result.Error = fmt.Errorf("emulator launch failed: %w", err)
	}
	return result, err
}

// WaitForBoot polls `getprop sys.boot_completed` via adb until it
// returns "1" or the timeout elapses. Returns the elapsed duration.
//
// The poll interval is 5 seconds (matches Lava's
// scripts/run-emulator-tests.sh contract before this package shipped,
// so the new package does not change observable behaviour).
func (a *AndroidEmulator) WaitForBoot(
	ctx context.Context,
	port int,
	timeout time.Duration,
) (time.Duration, error) {
	startedAt := time.Now()
	deadline := startedAt.Add(timeout)
	target := fmt.Sprintf("localhost:%d", port)

	// Forensic anchor (2026-05-04 evening): the previous form called
	// `adb connect localhost:<port>` ONCE before the poll loop.
	// On cold boot the emulator's ADB socket is not ready for ~30-60s
	// after the emulator process starts, so the pre-loop connect failed
	// silently (its err was discarded with `_, _`). Subsequent
	// `adb -s localhost:<port> shell getprop` calls then all returned
	// "device not found", the loop swallowed those errors as expected
	// "boot not yet ready" signals, and the timeout fired even though
	// the emulator booted successfully a few minutes in. Recorded as a
	// 6.A real-binary contract bug class — script's expectation of the
	// adb binary did not match the binary's reality.
	//
	// Fix: retry `adb connect` on every poll iteration. Connect is
	// idempotent (returns "already connected to ..." on second+ call)
	// so retrying carries no cost. The first iteration after the ADB
	// socket comes up actually establishes the connection; subsequent
	// `-s` calls then succeed and the boot-completed prop is read.
	for time.Now().Before(deadline) {
		_, _ = a.executor.Execute(ctx, a.adbBinary(), "connect", target)
		out, err := a.executor.Execute(
			ctx, a.adbBinary(), "-s", target,
			"shell", "getprop", "sys.boot_completed",
		)
		if err == nil && strings.TrimSpace(string(out)) == "1" {
			return time.Since(startedAt), nil
		}
		select {
		case <-ctx.Done():
			return time.Since(startedAt), ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return time.Since(startedAt),
		fmt.Errorf("boot not completed within %s", timeout)
}

// Install installs the APK on the running emulator via `adb -s
// localhost:<port> install -r <apkPath>`.
func (a *AndroidEmulator) Install(
	ctx context.Context,
	port int,
	apkPath string,
) error {
	if _, err := os.Stat(apkPath); err != nil {
		return fmt.Errorf("apk not found at %s: %w", apkPath, err)
	}
	target := fmt.Sprintf("localhost:%d", port)
	out, err := a.executor.Execute(
		ctx, a.adbBinary(), "-s", target, "install", "-r", apkPath,
	)
	if err != nil {
		return fmt.Errorf("adb install failed: %w; output=%s", err, out)
	}
	if !strings.Contains(string(out), "Success") {
		return fmt.Errorf("adb install reported non-Success output: %s", out)
	}
	return nil
}

// RunInstrumentation runs `connectedDebugAndroidTest` for the named
// test class via gradle. The runner expects to be invoked from a
// project root that has a gradlew + the matching `:app:connected*`
// task wired (Lava's case). The current implementation shells out via
// gradlew; future versions MAY drive `adb shell am instrument`
// directly for less wrapper overhead.
func (a *AndroidEmulator) RunInstrumentation(
	ctx context.Context,
	port int,
	testClass string,
	timeout time.Duration,
) (string, bool, error) {
	if testClass == "" {
		return "", false, fmt.Errorf("testClass MUST be non-empty")
	}
	target := fmt.Sprintf("localhost:%d", port)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(
		runCtx, "./gradlew",
		":app:connectedDebugAndroidTest",
		"-Pandroid.testInstrumentationRunnerArguments.class="+testClass,
		"--no-daemon",
	)
	cmd.Env = append(os.Environ(), "ANDROID_SERIAL="+target)
	out, err := cmd.CombinedOutput()
	output := string(out)
	passed := err == nil && strings.Contains(output, "BUILD SUCCESSFUL")
	if !passed && err == nil {
		err = fmt.Errorf("gradle exit zero but BUILD SUCCESSFUL not in output")
	}
	return output, passed, err
}

// Teardown stops the emulator via `adb -s localhost:<port> emu kill`.
// Returns nil on success.
func (a *AndroidEmulator) Teardown(ctx context.Context, port int) error {
	target := fmt.Sprintf("localhost:%d", port)
	out, err := a.executor.Execute(
		ctx, a.adbBinary(), "-s", target, "emu", "kill",
	)
	if err != nil {
		return fmt.Errorf("adb emu kill failed: %w; output=%s", err, out)
	}
	return nil
}
