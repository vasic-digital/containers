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

// emulatorSerials parses `adb devices` output and returns the set of
// emulator console ports currently registered (e.g. emulator-5554 →
// {5554}). Used by Boot() to discover the port the newly-launched
// emulator actually binds to. Multi-AVD matrix runs MUST NOT assume
// every emulator lands on 5554/5555 — when a previous emulator's
// Teardown is still in flight (or failed silently), the next launch
// lands on 5556/5557, 5558/5559, etc.
//
// Forensic anchor (2026-05-04 evening, exposed by ultrathink-driven
// diagnostic instrumentation): the prior Boot() hardcoded ADBPort=5555
// regardless of actual binding, causing every iteration of a multi-AVD
// matrix to test against whichever emulator happened to bind 5554/5555
// FIRST — the subsequent AVDs' emulators silently ran their tests
// against the FIRST AVD's process, then died at the next Teardown
// call. Recorded as a clause-6.I clause-7 architecture bluff.
func (a *AndroidEmulator) emulatorSerials(ctx context.Context) (map[int]bool, error) {
	out, err := a.executor.Execute(ctx, a.adbBinary(), "devices")
	if err != nil {
		return nil, fmt.Errorf("adb devices failed: %w", err)
	}
	serials := make(map[int]bool)
	for _, line := range strings.Split(string(out), "\n") {
		// Lines look like:
		//   emulator-5554\tdevice
		//   emulator-5556\toffline
		//   localhost:5555\tdevice          (ignore — that's a network alias)
		// We capture every emulator-<port> regardless of state, because
		// even an offline emulator is taking up that port.
		if !strings.HasPrefix(line, "emulator-") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		var port int
		if _, scanErr := fmt.Sscanf(fields[0], "emulator-%d", &port); scanErr == nil && port > 0 {
			serials[port] = true
		}
	}
	return serials, nil
}

// discoverNewSerial polls `adb devices` until a console port appears
// that wasn't in `before`, or the timeout elapses. The returned port
// is the CONSOLE port (e.g. 5554); callers compute ADB port = console + 1.
func (a *AndroidEmulator) discoverNewSerial(
	ctx context.Context,
	before map[int]bool,
	timeout time.Duration,
) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current, err := a.emulatorSerials(ctx)
		if err == nil {
			for port := range current {
				if !before[port] {
					return port, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("emulator port discovery cancelled: %w", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
	return 0, fmt.Errorf("no new emulator serial appeared in adb devices within %s", timeout)
}

// Boot starts the AVD in headless mode. The emulator process runs
// asynchronously; this method returns once the new emulator's serial
// is observable in `adb devices` (typically 1-3 seconds after the
// underlying QEMU process binds its sockets) — NOT once Android has
// fully booted. Use WaitForBoot to wait for sys.boot_completed=1.
//
// Per clause 6.I clause 6, coldBoot=true SHOULD be used for any gating
// matrix run — it disables snapshot reload, ensuring reproducibility
// across runs.
//
// Boot dynamically discovers the console/ADB port the new emulator
// binds to by diffing `adb devices` before and after the launch. This
// is the constitutional fix for the 2026-05-04 ultrathink-discovered
// bluff (see emulatorSerials KDoc above). Without dynamic discovery,
// multi-AVD matrix runs silently test against the FIRST emulator
// every iteration.
func (a *AndroidEmulator) Boot(
	ctx context.Context,
	avd AVD,
	coldBoot bool,
) (BootResult, error) {
	// Snapshot existing emulator ports BEFORE launch so we can detect
	// the new one after launch. Errors here are non-fatal — empty map
	// is a safe baseline (we'll just claim the first emulator we see).
	before, _ := a.emulatorSerials(ctx)

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
	if err := a.executor.Start(ctx, a.emulatorBinary(), args...); err != nil {
		return BootResult{
			AVD:          avd,
			Started:      false,
			BootDuration: time.Since(startedAt),
			Error:        fmt.Errorf("emulator launch failed: %w", err),
		}, err
	}

	// Discover the actual port the new emulator bound to. Bounded by a
	// 60s timeout — if adb doesn't see the new emulator within that,
	// something is structurally wrong (kvm denied, zygote crash, etc.)
	// and we fail loudly rather than silently mis-target later calls.
	newPort, derr := a.discoverNewSerial(ctx, before, 60*time.Second)
	if derr != nil {
		return BootResult{
			AVD:          avd,
			Started:      true,
			BootDuration: time.Since(startedAt),
			Error:        fmt.Errorf("emulator port discovery failed: %w", derr),
		}, derr
	}

	return BootResult{
		AVD:          avd,
		Started:      true,
		BootDuration: time.Since(startedAt),
		ConsolePort:  newPort,
		ADBPort:      newPort + 1,
	}, nil
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
//
// Diagnostic instrumentation (clause 6.I clause 7 forensics): before
// kicking off the test we log adb-devices state + the device's
// ro.product.model so a future operator can verify the test ran
// against the AVD the matrix runner intended.
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

	// Forensic diagnostics — see clause 6.I architecture audit.
	devicesOut, _ := a.executor.Execute(ctx, a.adbBinary(), "devices")
	sdkOut, _ := a.executor.Execute(
		ctx, a.adbBinary(), "-s", target,
		"shell", "getprop", "ro.build.version.sdk",
	)
	deviceOut, _ := a.executor.Execute(
		ctx, a.adbBinary(), "-s", target,
		"shell", "getprop", "ro.product.device",
	)
	fmt.Fprintf(os.Stderr,
		"[matrix-diag] target=%s sdk=%q device=%q\n",
		target,
		strings.TrimSpace(string(sdkOut)),
		strings.TrimSpace(string(deviceOut)),
	)
	fmt.Fprintf(os.Stderr,
		"[matrix-diag-devices] %s\n",
		strings.ReplaceAll(strings.TrimSpace(string(devicesOut)), "\n", " | "),
	)

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
