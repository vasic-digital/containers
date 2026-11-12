package emulator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.containers/pkg/hostbudget"
)

func TestG2R2_RequireKVM_RefusesWithoutKVM_NothingStarted(t *testing.T) {
	old := kvmDevicePath
	kvmDevicePath = filepath.Join(t.TempDir(), "no-kvm")
	defer func() { kvmDevicePath = old }()
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	admitted := false
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.RequireKVM = true
		cfg.Admit = func(n hostbudget.Need) hostbudget.Evidence {
			admitted = true
			return hostbudget.Evidence{Verdict: hostbudget.Fits}
		}
	})
	_, err := c.Boot(context.Background(), AVD{Name: "x"}, true)
	if !errors.Is(err, ErrKVMRequired) {
		t.Fatalf("want ErrKVMRequired, got %v", err)
	}
	if admitted || len(runCalls(fake)) != 0 {
		t.Errorf("nothing may be admitted or started (admitted=%v runs=%d)", admitted, len(runCalls(fake)))
	}
}

func TestG2R2_RequireKVM_PresentNodeAdmissionNeedsKVM(t *testing.T) {
	// With RequireKVM and a present node the admission need must carry NeedsKVM.
	old := kvmDevicePath
	f := filepath.Join(t.TempDir(), "kvm")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	kvmDevicePath = f
	defer func() { kvmDevicePath = old }()
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	var got hostbudget.Need
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.RequireKVM = true
		cfg.Admit = func(n hostbudget.Need) hostbudget.Evidence {
			got = n
			return hostbudget.Evidence{Verdict: hostbudget.DoesNotFit, Reasons: []string{"x"}}
		}
	})
	_, _ = c.Boot(context.Background(), AVD{Name: "x"}, true)
	if !got.NeedsKVM {
		t.Errorf("RequireKVM must set NeedsKVM in the admission need: %+v", got)
	}
}

func TestG2R2_EvidenceWriteFailureIsAnErrorAndNothingStarts(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	bad := filepath.Join(t.TempDir(), "file-not-dir")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.EvidenceDir = bad // a file: MkdirAll/OpenFile under it must fail
	})
	res, err := c.Boot(context.Background(), AVD{Name: "x"}, true)
	if err == nil || !errors.Is(err, ErrEvidenceWrite) {
		t.Fatalf("want ErrEvidenceWrite, got %v", err)
	}
	if res.Started || len(runCalls(fake)) != 0 {
		t.Errorf("nothing may start when the evidence cannot be written: %+v runs=%d", res, len(runCalls(fake)))
	}
}
