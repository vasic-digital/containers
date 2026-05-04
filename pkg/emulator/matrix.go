package emulator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// RunMatrix executes the matrix sequentially. If any AVD fails to
// boot, its TestResult is recorded as Passed=false with a descriptive
// error; the matrix continues to the next AVD (per clause 6.I clause 4
// "missing rows are missing evidence" — every AVD MUST get a row).
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

	result := MatrixResult{
		Config:    config,
		StartedAt: time.Now(),
	}
	for _, avd := range config.AVDs {
		boot, err := r.emulator.Boot(ctx, avd, config.ColdBoot)
		// We append the Boot result LATER (after WaitForBoot decides
		// whether BootCompleted is true) so AllPassed() reflects
		// reality. A Boot row with Started=true but
		// BootCompleted=false is the user-visible "process started but
		// Android never came up" outcome that clause 6.B explicitly
		// flags as un-healthy.
		if err != nil {
			result.Boots = append(result.Boots, boot)
			result.Tests = append(result.Tests, TestResult{
				AVD:       avd,
				TestClass: config.TestClass,
				Started:   time.Now(),
				Passed:    false,
				Error:     fmt.Errorf("boot failed: %w", err),
			})
			continue
		}
		if _, err := r.emulator.WaitForBoot(ctx, boot.ADBPort, bootTimeout); err != nil {
			boot.Error = err
			result.Boots = append(result.Boots, boot)
			result.Tests = append(result.Tests, TestResult{
				AVD:       avd,
				TestClass: config.TestClass,
				Started:   time.Now(),
				Passed:    false,
				Error:     fmt.Errorf("wait-for-boot failed: %w", err),
			})
			_ = r.emulator.Teardown(ctx, boot.ADBPort)
			continue
		}
		// Android booted (sys.boot_completed == 1). Mark the Boot row
		// completed and append. AllPassed() will see this as healthy.
		boot.BootCompleted = true
		result.Boots = append(result.Boots, boot)
		if err := r.emulator.Install(ctx, boot.ADBPort, config.APKPath); err != nil {
			result.Tests = append(result.Tests, TestResult{
				AVD:       avd,
				TestClass: config.TestClass,
				Started:   time.Now(),
				Passed:    false,
				Error:     fmt.Errorf("install failed: %w", err),
			})
			_ = r.emulator.Teardown(ctx, boot.ADBPort)
			continue
		}
		startedTest := time.Now()
		out, passed, runErr := r.emulator.RunInstrumentation(
			ctx, boot.ADBPort, config.TestClass, testTimeout,
		)
		result.Tests = append(result.Tests, TestResult{
			AVD:       avd,
			TestClass: config.TestClass,
			Started:   startedTest,
			Duration:  time.Since(startedTest),
			Passed:    passed,
			Output:    out,
			Error:     runErr,
		})
		_ = r.emulator.Teardown(ctx, boot.ADBPort)
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
		AVD          string  `json:"avd"`
		APILevel     int     `json:"api_level,omitempty"`
		FormFactor   string  `json:"form_factor,omitempty"`
		BootSeconds  float64 `json:"boot_seconds"`
		BootError    string  `json:"boot_error,omitempty"`
		TestClass    string  `json:"test_class"`
		TestPassed   bool    `json:"test_passed"`
		TestSeconds  float64 `json:"test_seconds"`
		TestError    string  `json:"test_error,omitempty"`
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
		rows = append(rows, rowJSON{
			AVD:         t.AVD.Name,
			APILevel:    t.AVD.APILevel,
			FormFactor:  t.AVD.FormFactor,
			BootSeconds: bootSec,
			BootError:   bootErr,
			TestClass:   t.TestClass,
			TestPassed:  t.Passed,
			TestSeconds: t.Duration.Seconds(),
			TestError:   testErr,
		})
	}
	doc := map[string]any{
		"started_at":  r.StartedAt.Format(time.RFC3339),
		"finished_at": r.FinishedAt.Format(time.RFC3339),
		"all_passed":  r.AllPassed(),
		"rows":        rows,
	}
	bytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o644)
}
