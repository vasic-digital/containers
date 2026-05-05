package emulator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AndroidMatrixRunner is the production [MatrixRunner] for Android
// AVDs. It uses an [Emulator] (typically [AndroidEmulator]) and walks
// the AVD list sequentially: cold-boot → install → run-tests →
// teardown → next AVD. Per clause 6.I clause 6, sequential execution
// avoids cross-AVD interference (a parallel runner would be a future
// optimisation gated on its own falsifiability rehearsal).
type AndroidMatrixRunner struct {
	emulator Emulator
}

// NewAndroidMatrixRunner constructs a runner backed by the supplied
// [Emulator]. Callers typically pass [NewAndroidEmulator] for
// production runs and [NewAndroidEmulatorWithExecutor] for tests.
func NewAndroidMatrixRunner(emulator Emulator) *AndroidMatrixRunner {
	return &AndroidMatrixRunner{emulator: emulator}
}

func defaultIfZero(d, fallback time.Duration) time.Duration {
	if d == 0 {
		return fallback
	}
	return d
}

// runOne executes one (boot → install → test → teardown) cycle for a
// single AVD. Returns the BootResult and TestResult ready to append to
// MatrixResult.Boots / .Tests. Captures diag pre-test and parses JUnit
// XML post-test per Group B.
//
// runOne is invoked sequentially in serial mode and concurrently from
// a worker pool when MatrixConfig.Concurrent > 1. Each invocation owns
// its own emulator instance, so concurrent calls are safe as long as
// the underlying Emulator implementation is too (AndroidEmulator
// satisfies that — Boot()'s discovery picks an unused console port).
func (r *AndroidMatrixRunner) runOne(
	ctx context.Context,
	avd AVD,
	config MatrixConfig,
	bootTimeout time.Duration,
	testTimeout time.Duration,
) (BootResult, TestResult) {
	boot, err := r.emulator.Boot(ctx, avd, config.ColdBoot)
	if err != nil {
		return boot, TestResult{
			AVD:        avd,
			TestClass:  config.TestClass,
			Started:    time.Now(),
			Passed:     false,
			Error:      fmt.Errorf("boot failed: %w", err),
			Concurrent: maxInt(config.Concurrent, 1),
		}
	}
	waitDuration, err := r.emulator.WaitForBoot(ctx, boot.ADBPort, bootTimeout)
	boot.BootDuration += waitDuration
	if err != nil {
		boot.Error = err
		_ = r.emulator.Teardown(ctx, boot.ADBPort)
		return boot, TestResult{
			AVD:        avd,
			TestClass:  config.TestClass,
			Started:    time.Now(),
			Passed:     false,
			Error:      fmt.Errorf("wait-for-boot failed: %w", err),
			Concurrent: maxInt(config.Concurrent, 1),
		}
	}
	boot.BootCompleted = true

	if err := r.emulator.Install(ctx, boot.ADBPort, config.APKPath); err != nil {
		_ = r.emulator.Teardown(ctx, boot.ADBPort)
		return boot, TestResult{
			AVD:        avd,
			TestClass:  config.TestClass,
			Started:    time.Now(),
			Passed:     false,
			Error:      fmt.Errorf("install failed: %w", err),
			Concurrent: maxInt(config.Concurrent, 1),
		}
	}

	// Diagnostic capture happens here, between Install and the test —
	// after Android is up and the APK is installed (so the emulator is
	// stable) and before the test runs (so the diag reflects the
	// state the test will encounter).
	diag := r.captureDiagnostic(ctx, boot.ADBPort, avd)

	startedTest := time.Now()
	out, passed, runErr := r.emulator.RunInstrumentation(
		ctx, boot.ADBPort, config.TestClass, testTimeout,
	)
	test := TestResult{
		AVD:        avd,
		TestClass:  config.TestClass,
		Started:    startedTest,
		Duration:   time.Since(startedTest),
		Passed:     passed,
		Output:     out,
		Error:      runErr,
		Diag:       diag,
		Concurrent: maxInt(config.Concurrent, 1),
	}

	// Persist per-AVD evidence (gradle.log + JUnit XML test-report).
	avdDir := filepath.Join(config.EvidenceDir, avd.Name)
	if mkErr := os.MkdirAll(avdDir, 0o755); mkErr == nil {
		logPath := filepath.Join(avdDir, "gradle.log")
		if werr := os.WriteFile(logPath, []byte(out), 0o644); werr != nil {
			fmt.Fprintf(os.Stderr,
				"[matrix] warning: failed to persist gradle.log for %s: %v\n",
				avd.Name, werr,
			)
		}
		matches, _ := filepath.Glob("app/build/outputs/androidTest-results/connected/debug/TEST-*.xml")
		if len(matches) > 0 {
			reportDir := filepath.Join(avdDir, "test-report")
			_ = os.MkdirAll(reportDir, 0o755)
			for _, src := range matches {
				data, rerr := os.ReadFile(src)
				if rerr != nil {
					continue
				}
				dst := filepath.Join(reportDir, filepath.Base(src))
				_ = os.WriteFile(dst, data, 0o644)
			}
			// Parse JUnit failures from every copied XML; aggregate
			// across files (Gradle emits one XML per class).
			var summaries []FailureSummary
			reportEntries, _ := os.ReadDir(reportDir)
			for _, ent := range reportEntries {
				if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".xml") {
					continue
				}
				summaries = append(summaries, parseJUnitFailures(filepath.Join(reportDir, ent.Name()))...)
			}
			test.FailureSummaries = summaries
		}
	}
	if test.FailureSummaries == nil {
		test.FailureSummaries = []FailureSummary{}
	}
	_ = r.emulator.Teardown(ctx, boot.ADBPort)
	return boot, test
}

// captureDiagnostic gathers the per-AVD forensic snapshot used by Group
// B's clause 6.I extension. Best-effort: missing fields default to zero
// values so a partial diag is recorded rather than no diag at all.
func (r *AndroidMatrixRunner) captureDiagnostic(ctx context.Context, port int, avd AVD) DiagnosticInfo {
	d := DiagnosticInfo{}
	if ae, ok := r.emulator.(*AndroidEmulator); ok {
		target := fmt.Sprintf("localhost:%d", port)
		if sdkOut, err := ae.executor.Execute(ctx, ae.adbBinary(), "-s", target, "shell", "getprop", "ro.build.version.sdk"); err == nil {
			if sdk, perr := strconv.Atoi(strings.TrimSpace(string(sdkOut))); perr == nil {
				d.SDK = sdk
			}
		}
		if modelOut, err := ae.executor.Execute(ctx, ae.adbBinary(), "-s", target, "shell", "getprop", "ro.product.model"); err == nil && strings.TrimSpace(string(modelOut)) != "" {
			d.Device = strings.TrimSpace(string(modelOut))
		} else if devOut, err := ae.executor.Execute(ctx, ae.adbBinary(), "-s", target, "shell", "getprop", "ro.product.device"); err == nil {
			d.Device = strings.TrimSpace(string(devOut))
		}
		if devicesOut, err := ae.executor.Execute(ctx, ae.adbBinary(), "devices", "-l"); err == nil {
			for _, line := range strings.Split(string(devicesOut), "\n") {
				if strings.HasPrefix(line, target) {
					d.ADBDevicesState = strings.TrimSpace(line)
					break
				}
			}
		}
		// Target (system-images package id) — best-effort via
		// avdmanager. The CLI is `avdmanager list avd -c` for compact
		// output; falling back to empty is acceptable.
		avdmanager := filepath.Join(ae.androidSdkRoot, "cmdline-tools", "latest", "bin", "avdmanager")
		if avdmanagerOut, err := ae.executor.Execute(ctx, avdmanager, "list", "avd"); err == nil {
			d.Target = parseAVDTarget(string(avdmanagerOut), avd.Name)
		}
	}
	return d
}

// parseAVDTarget walks `avdmanager list avd` text output and finds the
// "Based on:" line for the AVD with the requested name. Returns empty
// string when not found.
func parseAVDTarget(out string, avdName string) string {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		nameField := strings.TrimSpace(line)
		if !strings.HasPrefix(nameField, "Name:") {
			continue
		}
		got := strings.TrimSpace(strings.TrimPrefix(nameField, "Name:"))
		if got != avdName {
			continue
		}
		// Look for "Based on: <package id>" within the next 10 lines
		for j := i + 1; j < len(lines) && j < i+10; j++ {
			tl := strings.TrimSpace(lines[j])
			if strings.HasPrefix(tl, "Based on:") {
				return strings.TrimSpace(strings.TrimPrefix(tl, "Based on:"))
			}
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RunMatrix executes the matrix. Serial by default (Concurrent<=1);
// dispatches to a bounded worker pool when Concurrent>1. If any AVD
// fails to boot, its TestResult is recorded as Passed=false with a
// descriptive error; the matrix continues to the next AVD (per clause
// 6.I clause 4 "missing rows are missing evidence" — every AVD MUST
// get a row).
//
// MatrixResult.Gating is true ONLY when Concurrent==1 AND Dev==false —
// any other configuration produces a non-gating run that scripts/tag.sh
// MUST refuse.
//
// On exit RunMatrix writes a machine-readable attestation file at
// EvidenceDir/real-device-verification.json. Operators may also
// generate the human-readable .md form from this file.
func (r *AndroidMatrixRunner) RunMatrix(
	ctx context.Context,
	config MatrixConfig,
) (MatrixResult, error) {
	if len(config.AVDs) == 0 {
		return MatrixResult{}, fmt.Errorf("MatrixConfig.AVDs is empty")
	}
	if config.APKPath == "" {
		return MatrixResult{}, fmt.Errorf("MatrixConfig.APKPath is empty")
	}
	if config.TestClass == "" {
		return MatrixResult{}, fmt.Errorf("MatrixConfig.TestClass is empty")
	}
	if config.EvidenceDir == "" {
		return MatrixResult{}, fmt.Errorf("MatrixConfig.EvidenceDir is empty")
	}
	if err := os.MkdirAll(config.EvidenceDir, 0o755); err != nil {
		return MatrixResult{}, fmt.Errorf("create evidence dir: %w", err)
	}

	bootTimeout := defaultIfZero(config.BootTimeout, 5*time.Minute)
	testTimeout := defaultIfZero(config.TestTimeout, 10*time.Minute)
	concurrent := config.Concurrent
	if concurrent < 1 {
		concurrent = 1
	}

	result := MatrixResult{
		Config:    config,
		StartedAt: time.Now(),
		Gating:    concurrent == 1 && !config.Dev,
	}

	if concurrent == 1 {
		// Serial path — preserves existing behaviour.
		for _, avd := range config.AVDs {
			boot, test := r.runOne(ctx, avd, config, bootTimeout, testTimeout)
			result.Boots = append(result.Boots, boot)
			result.Tests = append(result.Tests, test)
		}
	} else {
		// Concurrent path — bounded worker pool. Each worker pulls
		// AVDs off a channel and runs runOne; results land on a
		// buffered channel that is drained after all workers exit.
		type pair struct {
			boot BootResult
			test TestResult
		}
		queue := make(chan AVD)
		results := make(chan pair, len(config.AVDs))
		var wg sync.WaitGroup
		for w := 0; w < concurrent; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for avd := range queue {
					boot, test := r.runOne(ctx, avd, config, bootTimeout, testTimeout)
					results <- pair{boot: boot, test: test}
				}
			}()
		}
		for _, avd := range config.AVDs {
			queue <- avd
		}
		close(queue)
		wg.Wait()
		close(results)
		for p := range results {
			result.Boots = append(result.Boots, p.boot)
			result.Tests = append(result.Tests, p.test)
		}
	}

	result.FinishedAt = time.Now()
	attestationFile := filepath.Join(config.EvidenceDir, "real-device-verification.json")
	if err := writeAttestation(attestationFile, result); err == nil {
		result.AttestationFile = attestationFile
	}
	return result, nil
}

func writeAttestation(path string, r MatrixResult) error {
	type rowJSON struct {
		AVD              string           `json:"avd"`
		APILevel         int              `json:"api_level,omitempty"`
		FormFactor       string           `json:"form_factor,omitempty"`
		BootSeconds      float64          `json:"boot_seconds"`
		BootError        string           `json:"boot_error,omitempty"`
		TestClass        string           `json:"test_class"`
		TestPassed       bool             `json:"test_passed"`
		TestSeconds      float64          `json:"test_seconds"`
		TestError        string           `json:"test_error,omitempty"`
		GradleLogPath    string           `json:"gradle_log_path,omitempty"`
		Diag             DiagnosticInfo   `json:"diag"`
		FailureSummaries []FailureSummary `json:"failure_summaries"`
		Concurrent       int              `json:"concurrent"`
	}
	rows := make([]rowJSON, 0, len(r.Tests))
	for i, t := range r.Tests {
		var bootSec float64
		var bootErr string
		if i < len(r.Boots) {
			bootSec = r.Boots[i].BootDuration.Seconds()
			if r.Boots[i].Error != nil {
				bootErr = r.Boots[i].Error.Error()
			}
		}
		var testErr string
		if t.Error != nil {
			testErr = t.Error.Error()
		}
		summaries := t.FailureSummaries
		if summaries == nil {
			summaries = []FailureSummary{}
		}
		concurrent := t.Concurrent
		if concurrent < 1 {
			concurrent = 1
		}
		rows = append(rows, rowJSON{
			AVD:              t.AVD.Name,
			APILevel:         t.AVD.APILevel,
			FormFactor:       t.AVD.FormFactor,
			BootSeconds:      bootSec,
			BootError:        bootErr,
			TestClass:        t.TestClass,
			TestPassed:       t.Passed,
			TestSeconds:      t.Duration.Seconds(),
			TestError:        testErr,
			GradleLogPath:    filepath.Join(t.AVD.Name, "gradle.log"),
			Diag:             t.Diag,
			FailureSummaries: summaries,
			Concurrent:       concurrent,
		})
	}
	doc := map[string]any{
		"started_at":  r.StartedAt.Format(time.RFC3339),
		"finished_at": r.FinishedAt.Format(time.RFC3339),
		"all_passed":  r.AllPassed(),
		"gating":      r.Gating,
		"rows":        rows,
	}
	bytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o644)
}

// parseJUnitFailures reads a single JUnit XML report file and returns
// every <failure> and <error> child of every <testcase> as a
// FailureSummary. Tolerates:
//   - Multiple <testsuite> siblings (Gradle's per-class XML output).
//   - <testcase> with both <failure> and <error> children — both are
//     captured as separate entries.
//   - <failure> / <error> with no message attribute (empty Message).
//   - <failure> / <error> with no text content (empty Body).
//
// Malformed XML returns a single synthetic FailureSummary with
// Type="<unparseable>". The synthetic entry is evidence corruption,
// NOT a row failure — the gating signal stays on TestResult.Passed
// per Sixth Law clause 3.
func parseJUnitFailures(xmlPath string) []FailureSummary {
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return []FailureSummary{{
			Type:    "<unparseable>",
			Message: fmt.Sprintf("junit-xml read failed: %v", err),
		}}
	}
	type junitFailure struct {
		Message string `xml:"message,attr"`
		Type    string `xml:"type,attr"`
		Body    string `xml:",chardata"`
	}
	type junitTestcase struct {
		Class    string         `xml:"classname,attr"`
		Name     string         `xml:"name,attr"`
		Failures []junitFailure `xml:"failure"`
		Errors   []junitFailure `xml:"error"`
	}
	type junitTestsuite struct {
		Testcases []junitTestcase `xml:"testcase"`
	}
	type junitTestsuites struct {
		Suites []junitTestsuite `xml:"testsuite"`
	}
	// Decode into either <testsuites> or a single <testsuite>.
	var suites []junitTestsuite
	var ts junitTestsuites
	if err := xml.Unmarshal(data, &ts); err == nil && len(ts.Suites) > 0 {
		suites = ts.Suites
	} else {
		var single junitTestsuite
		if err := xml.Unmarshal(data, &single); err != nil {
			return []FailureSummary{{
				Type:    "<unparseable>",
				Message: fmt.Sprintf("junit-xml parse failed: %v", err),
			}}
		}
		suites = []junitTestsuite{single}
	}
	var out []FailureSummary
	for _, suite := range suites {
		for _, tc := range suite.Testcases {
			for _, f := range tc.Failures {
				out = append(out, FailureSummary{
					Class:   tc.Class,
					Name:    tc.Name,
					Type:    "failure",
					Message: f.Message,
					Body:    f.Body,
				})
			}
			for _, e := range tc.Errors {
				out = append(out, FailureSummary{
					Class:   tc.Class,
					Name:    tc.Name,
					Type:    "error",
					Message: e.Message,
					Body:    e.Body,
				})
			}
		}
	}
	if out == nil {
		return []FailureSummary{}
	}
	return out
}
