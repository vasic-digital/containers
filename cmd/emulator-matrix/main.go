// cmd/emulator-matrix — multi-AVD matrix runner for Android Compose UI
// tests. Constitutional anchor: parent-Lava clause 6.I (Multi-Emulator
// Container Matrix as Real-Device Equivalent) + clause 6.K
// (Builds-Inside-Containers Mandate). Lava's
// scripts/run-emulator-tests.sh becomes thin glue invoking this CLI
// after this package ships.
//
// Usage:
//
//   emulator-matrix \
//     --android-sdk-root /opt/android-sdk \
//     --apk releases/1.2.1/android-debug/app-debug.apk \
//     --test-class lava.app.challenges.Challenge01AppLaunchAndTrackerSelectionTest \
//     --evidence-dir .lava-ci-evidence/Lava-1.2.2 \
//     --avds CZ_API28_Phone,CZ_API30_Phone,CZ_API34_Phone,Pixel_9a \
//     --cold-boot
//
// Each comma-separated AVD entry MAY include the API level after a
// colon: `Pixel_9a:36:phone` (name:apiLevel:formFactor). The api level
// + form factor are recorded in the attestation file per clause 6.I
// clause 4.
//
// Exit codes:
//   0 — every AVD booted, every test passed (matrix attestation green)
//   1 — at least one AVD failed boot OR at least one test failed
//   2 — invalid CLI arguments OR the runner errored before producing
//       any attestation rows
//
// Anti-bluff posture (clauses 6.J/6.L): a CLI that exits 0 when ANY
// AVD failed would be a bluff. The exit-code logic above means
// `tag.sh` can rely on `exit 0 ⇒ matrix passed`.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"digital.vasic.containers/pkg/emulator"
)

func parseAVDs(spec string) ([]emulator.AVD, error) {
	if spec == "" {
		return nil, fmt.Errorf("--avds MUST be a non-empty comma-separated list")
	}
	parts := strings.Split(spec, ",")
	avds := make([]emulator.AVD, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.Split(p, ":")
		avd := emulator.AVD{Name: fields[0]}
		if len(fields) > 1 {
			lvl, err := strconv.Atoi(strings.TrimSpace(fields[1]))
			if err != nil {
				return nil, fmt.Errorf("AVD %q: invalid api level %q", p, fields[1])
			}
			avd.APILevel = lvl
		}
		if len(fields) > 2 {
			avd.FormFactor = strings.TrimSpace(fields[2])
		}
		avds = append(avds, avd)
	}
	if len(avds) == 0 {
		return nil, fmt.Errorf("--avds parsed to zero AVDs")
	}
	return avds, nil
}

func main() {
	flagSdkRoot := flag.String("android-sdk-root", os.Getenv("ANDROID_SDK_ROOT"),
		"Host path to the Android SDK (default $ANDROID_SDK_ROOT)")
	flagAPK := flag.String("apk", "", "Host path to the debug APK to install")
	flagTestClass := flag.String("test-class", "",
		"Fully-qualified instrumentation test class")
	flagEvidence := flag.String("evidence-dir", "",
		"Where to write per-AVD attestation rows (real-device-verification.json)")
	flagAVDs := flag.String("avds", "",
		"Comma-separated AVD list; entries MAY be 'Name:APILevel:FormFactor'")
	flagColdBoot := flag.Bool("cold-boot", true,
		"Disable snapshot reload (clause 6.I clause 6 — gating runs MUST cold-boot)")
	flagBootTimeout := flag.Duration("boot-timeout", 5*time.Minute,
		"Per-AVD cold-boot timeout")
	flagTestTimeout := flag.Duration("test-timeout", 10*time.Minute,
		"Per-test execution timeout")
	flag.Parse()

	if *flagAPK == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --apk is required")
		os.Exit(2)
	}
	if *flagTestClass == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --test-class is required")
		os.Exit(2)
	}
	if *flagEvidence == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --evidence-dir is required")
		os.Exit(2)
	}
	if *flagSdkRoot == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --android-sdk-root or $ANDROID_SDK_ROOT is required")
		os.Exit(2)
	}
	avds, err := parseAVDs(*flagAVDs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	emu := emulator.NewAndroidEmulator(*flagSdkRoot)
	runner := emulator.NewAndroidMatrixRunner(emu)
	result, err := runner.RunMatrix(ctx, emulator.MatrixConfig{
		AVDs:           avds,
		AndroidSdkRoot: *flagSdkRoot,
		APKPath:        *flagAPK,
		TestClass:      *flagTestClass,
		EvidenceDir:    *flagEvidence,
		BootTimeout:    *flagBootTimeout,
		TestTimeout:    *flagTestTimeout,
		ColdBoot:       *flagColdBoot,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: matrix runner failed: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("Matrix run finished. Attestation: %s\n", result.AttestationFile)
	for i, t := range result.Tests {
		status := "PASS"
		if !t.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%d] %-25s api=%d %s %s\n",
			i+1, t.AVD.Name, t.AVD.APILevel, t.AVD.FormFactor, status)
		if t.Error != nil {
			fmt.Printf("       error: %v\n", t.Error)
		}
	}
	if !result.AllPassed() {
		fmt.Fprintln(os.Stderr,
			"MATRIX FAILED — at least one AVD did not pass. tag.sh MUST refuse this commit.")
		os.Exit(1)
	}
	fmt.Println("MATRIX PASSED — every AVD booted and every test passed.")
}
