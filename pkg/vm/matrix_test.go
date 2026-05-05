package vm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubVM is the matrix-runner test fake — mirror of stubEmulator from
// pkg/emulator's matrix_test.go.
type stubVM struct {
	port       int32
	bootError  error
	scriptExit int
	scriptOut  string
	scriptErr  string
}

func (s *stubVM) Boot(_ context.Context, cfg VMConfig) (BootResult, error) {
	if s.bootError != nil {
		return BootResult{Target: cfg.Target}, s.bootError
	}
	s.port += 2
	return BootResult{
		Target:       cfg.Target,
		Started:      true,
		SSHPort:      int(s.port),
		MonitorPort:  int(s.port + 1),
		BootDuration: 100 * time.Millisecond,
	}, nil
}
func (s *stubVM) WaitForReady(_ context.Context, _ int, _ time.Duration) error { return nil }
func (s *stubVM) Upload(_ context.Context, _ int, _, _ string) error           { return nil }
func (s *stubVM) Run(_ context.Context, _ int, _ string, _ map[string]string, _ time.Duration) (string, string, int, error) {
	return s.scriptOut, s.scriptErr, s.scriptExit, nil
}
func (s *stubVM) Download(_ context.Context, _ int, _, _ string) error { return nil }
func (s *stubVM) Teardown(_ context.Context, _, _ int) error           { return nil }

func writeManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vm-images.json")
	body := `{"version":1,"images":[{"id":"alpine-x86_64","url":"http://x","sha256":"` + strings.Repeat("a", 64) + `","size":1,"format":"qcow2"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func runMatrixWithStubVM(t *testing.T, concurrent int, dev bool, scriptExit int) VMMatrixResult {
	t.Helper()
	manifest := writeManifest(t)
	dir := t.TempDir()
	r := NewQEMUMatrixRunner(&stubVM{scriptExit: scriptExit}, nil)
	res, err := r.RunMatrix(context.Background(), VMMatrixConfig{
		Targets: []VMTarget{
			{ID: "alpine-x86_64", Arch: "x86_64", Distro: "alpine"},
		},
		Script:        "/tmp/script.sh",
		EvidenceDir:   dir,
		Concurrent:    concurrent,
		Dev:           dev,
		ImageManifest: manifest,
	})
	if err != nil {
		t.Fatalf("RunMatrix: %v", err)
	}
	return res
}

func TestQEMUMatrixRunner_AllPass_GatingTrue(t *testing.T) {
	res := runMatrixWithStubVM(t, 1, false, 0)
	if !res.Gating {
		t.Fatalf("expected Gating=true on serial+non-dev, got false")
	}
	if !res.AllPassed() {
		t.Fatalf("expected AllPassed=true, got false")
	}
	if len(res.Rows) != 1 || res.Rows[0].Concurrent != 1 {
		t.Fatalf("rows: %+v", res.Rows)
	}
}

func TestQEMUMatrixRunner_Gating_FalseOnConcurrent(t *testing.T) {
	res := runMatrixWithStubVM(t, 2, false, 0)
	if res.Gating {
		t.Fatalf("expected Gating=false when Concurrent=2, got true")
	}
}

func TestQEMUMatrixRunner_Gating_FalseOnDev(t *testing.T) {
	res := runMatrixWithStubVM(t, 1, true, 0)
	if res.Gating {
		t.Fatalf("expected Gating=false when Dev=true, got true")
	}
}

func TestQEMUMatrixRunner_ScriptNonZeroExit_RowFails(t *testing.T) {
	res := runMatrixWithStubVM(t, 1, false, 7) // exit code 7
	if res.Rows[0].Passed {
		t.Fatalf("expected row Passed=false on script exit=7, got true")
	}
	if len(res.Rows[0].FailureSummaries) == 0 {
		t.Fatalf("expected at least one FailureSummary capturing exit=7")
	}
	if res.AllPassed() {
		t.Fatalf("AllPassed should be false")
	}
}

func TestQEMUMatrixRunner_BootFailure_RowFails(t *testing.T) {
	manifest := writeManifest(t)
	dir := t.TempDir()
	stub := &stubVM{bootError: errors.New("kvm denied")}
	r := NewQEMUMatrixRunner(stub, nil)
	res, _ := r.RunMatrix(context.Background(), VMMatrixConfig{
		Targets:       []VMTarget{{ID: "alpine-x86_64"}},
		Script:        "/tmp/x",
		EvidenceDir:   dir,
		Concurrent:    1,
		ImageManifest: manifest,
	})
	if res.AllPassed() {
		t.Fatalf("AllPassed should be false on boot failure")
	}
	if !strings.Contains(res.Rows[0].BootError, "kvm denied") {
		t.Fatalf("BootError missing the kvm-denied substring: %q", res.Rows[0].BootError)
	}
}

func TestQEMUMatrixRunner_AttestationSchema_HasGatingAndDiagAndConcurrent(t *testing.T) {
	res := runMatrixWithStubVM(t, 1, false, 0)
	if res.AttestationFile == "" {
		t.Fatalf("AttestationFile not set")
	}
	data, err := os.ReadFile(res.AttestationFile)
	if err != nil {
		t.Fatalf("read attestation: %v", err)
	}
	body := string(data)
	for _, want := range []string{`"gating": true`, `"diag":`, `"failure_summaries":`, `"concurrent":`} {
		if !strings.Contains(body, want) {
			t.Fatalf("attestation missing %q; full body:\n%s", want, body)
		}
	}
}
