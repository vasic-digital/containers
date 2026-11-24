package emulator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"digital.vasic.containers/pkg/hostbudget"
)

// G2 (CAT-DEVEMU) PROVISIONAL: host-budget admission before the emulator
// container is started, resource envelope on the run argv, and an
// open-for-write KVM probe instead of a stat. No container is started here.

func admitWith(v hostbudget.Verdict, reason string) func(hostbudget.Need) hostbudget.Evidence {
	return func(n hostbudget.Need) hostbudget.Evidence {
		return hostbudget.Evidence{Verdict: v, Reasons: []string{reason}, Need: n}
	}
}

func newG2Emu(t *testing.T, fake *fakeExecutor, mod func(*ContainerizedConfig)) *Containerized {
	t.Helper()
	cfg := ContainerizedConfig{RuntimeBinary: "podman", Image: "any:tag", Executor: fake}
	if mod != nil {
		mod(&cfg)
	}
	c, err := NewContainerized(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// runCalls returns the emulator container launches (`run -d ...`), not the
// throwaway AVD-listing probe.
func runCalls(f *fakeExecutor) []fakeCall {
	var out []fakeCall
	for _, c := range f.calls {
		if c.Name == "podman" && len(c.Args) > 1 && c.Args[0] == "run" && c.Args[1] == "-d" {
			out = append(out, c)
		}
	}
	return out
}

func flagVal(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func TestG2_Boot_RefusedWhenHostDoesNotFit_NothingStarted(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	dir := t.TempDir()
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.Admit = admitWith(hostbudget.DoesNotFit, "load: too high")
		cfg.EvidenceDir = dir
	})
	res, err := c.Boot(context.Background(), AVD{Name: "x"}, true)
	if err == nil {
		t.Fatal("Boot must refuse when the host budget does not fit")
	}
	if !errors.Is(err, ErrHostBudget) {
		t.Fatalf("error must wrap ErrHostBudget, got %v", err)
	}
	if res.Started {
		t.Error("Started must be false")
	}
	if n := len(runCalls(fake)); n != 0 {
		t.Fatalf("no `podman run` may be issued on refusal; got %d", n)
	}
	if !strings.Contains(err.Error(), "load: too high") {
		t.Errorf("error must carry the resolved reason: %v", err)
	}
	ents, _ := filepath.Glob(filepath.Join(dir, "hostbudget-*.json"))
	if len(ents) != 1 {
		t.Fatalf("exactly one evidence file expected, got %v", ents)
	}
	b, _ := os.ReadFile(ents[0])
	if !strings.Contains(string(b), `"does_not_fit"`) {
		t.Errorf("evidence must record the verdict: %s", b)
	}
}

func TestG2_Boot_RefusedWhenUndetermined(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.Admit = admitWith(hostbudget.Undetermined, "memory PSI unreadable")
	})
	_, err := c.Boot(context.Background(), AVD{Name: "x"}, true)
	if !errors.Is(err, ErrHostBudget) {
		t.Fatalf("undetermined must refuse (never read as fits): %v", err)
	}
	if len(runCalls(fake)) != 0 {
		t.Fatal("no run on undetermined")
	}
}

func TestG2_Boot_FitsStartsWithResourceEnvelope(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	var gotNeed hostbudget.Need
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.Memory = 3 << 30
		cfg.CPUs = 2
		cfg.PidsLimit = 1234
		cfg.Admit = func(n hostbudget.Need) hostbudget.Evidence {
			gotNeed = n
			return hostbudget.Evidence{Verdict: hostbudget.Fits}
		}
	})
	res, err := c.Boot(context.Background(), AVD{Name: "x"}, true)
	if err != nil || !res.Started {
		t.Fatalf("Boot: %v %+v", err, res)
	}
	runs := runCalls(fake)
	if len(runs) != 1 {
		t.Fatalf("one run expected, got %d", len(runs))
	}
	a := runs[0].Args
	if v, _ := flagVal(a, "--memory"); v != "3221225472" {
		t.Errorf("--memory = %q", v)
	}
	if v, _ := flagVal(a, "--memory-swap"); v != "3221225472" {
		t.Errorf("--memory-swap must equal memory (no swap), got %q", v)
	}
	if v, _ := flagVal(a, "--cpus"); v != "2" {
		t.Errorf("--cpus = %q", v)
	}
	if v, _ := flagVal(a, "--pids-limit"); v != "1234" {
		t.Errorf("--pids-limit = %q", v)
	}
	// envelope flags precede the end-of-options marker
	dd := -1
	for i, x := range a {
		if x == "--" {
			dd = i
		}
	}
	for _, f := range []string{"--memory", "--cpus", "--pids-limit"} {
		for i, x := range a {
			if x == f && dd >= 0 && i > dd {
				t.Errorf("%s after `--` would be parsed as image args", f)
			}
		}
	}
	if gotNeed.MemBytes != 3<<30 || gotNeed.CPUs != 2 || gotNeed.Pids != 1234 {
		t.Errorf("admission must be asked for the SAME envelope that is applied: %+v", gotNeed)
	}
}

func TestG2_Boot_DefaultEnvelopeIsHostAdaptiveAndAlwaysPresent(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.Admit = admitWith(hostbudget.Fits, "")
	})
	if _, err := c.Boot(context.Background(), AVD{Name: "x"}, true); err != nil {
		t.Fatal(err)
	}
	a := runCalls(fake)[0].Args
	for _, f := range []string{"--memory", "--memory-swap", "--cpus", "--pids-limit"} {
		if _, ok := flagVal(a, f); !ok {
			t.Errorf("default envelope must include %s: %v", f, a)
		}
	}
}

func TestG2_DefaultLimits_Formula(t *testing.T) {
	m, cpus, pids := defaultEmulatorLimits(16)
	if m != DefaultEmulatorMemoryBytes || cpus != 3 || pids != DefaultEmulatorPids {
		t.Errorf("16 cpus: %d %g %d", m, cpus, pids)
	}
	if _, cpus, _ := defaultEmulatorLimits(4); cpus != 1 {
		t.Errorf("4 cpus => 0.25*4 = 1, got %g", cpus)
	}
	if _, cpus, _ := defaultEmulatorLimits(2); cpus != 1 {
		t.Errorf("floor of 1 cpu, got %g", cpus)
	}
	if _, cpus, _ := defaultEmulatorLimits(64); cpus != 3 {
		t.Errorf("cap of 3 cpus, got %g", cpus)
	}
}

func TestG2_LegacyBuildArgsHaveNoEnvelope(t *testing.T) {
	// the pre-G2 signature keeps its exact output (existing callers/tests)
	a := buildContainerRunArgs("podman", "n", 6555, AVD{Name: "x"}, true, "img")
	for _, f := range []string{"--memory", "--cpus", "--pids-limit"} {
		if _, ok := flagVal(a, f); ok {
			t.Errorf("legacy builder must not add %s", f)
		}
	}
}

func TestG2_KVMProbe_OpenForWriteNotStat(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("SKIP-OK: root bypasses file modes")
	}
	ro := filepath.Join(t.TempDir(), "kvm")
	if err := os.WriteFile(ro, nil, 0o444); err != nil {
		t.Fatal(err)
	}
	withKVMDevicePath(t, ro)
	if kvmAvailable() {
		t.Fatal("a node that exists but cannot be opened read-write must NOT count as available (stat-only check regression)")
	}
	a := buildContainerRunArgs("podman", "n", 6555, AVD{Name: "x"}, true, "img")
	if argsContainDeviceKVM(a, ro) {
		t.Error("--device must not be passed for an unusable KVM node")
	}
	rw := filepath.Join(t.TempDir(), "kvm")
	if err := os.WriteFile(rw, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	withKVMDevicePath(t, rw)
	if !kvmAvailable() {
		t.Fatal("a usable node must count as available")
	}
}

func TestG2_Boot_AdmissionAsksForKVMOnlyWhenPresent(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	withKVMDevicePath(t, filepath.Join(t.TempDir(), "absent"))
	var need hostbudget.Need
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) {
		cfg.Admit = func(n hostbudget.Need) hostbudget.Evidence {
			need = n
			return hostbudget.Evidence{Verdict: hostbudget.Fits}
		}
	})
	if _, err := c.Boot(context.Background(), AVD{Name: "x"}, true); err != nil {
		t.Fatal(err)
	}
	if need.NeedsKVM {
		t.Error("KVM absent => TCG fallback; admission must not demand KVM")
	}
}

func TestG2_AVDListingProbeContainerCarriesEnvelope(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("id")}}}
	c := newG2Emu(t, fake, func(cfg *ContainerizedConfig) { cfg.Admit = admitWith(hostbudget.Fits, "") })
	if _, err := c.Boot(context.Background(), AVD{Name: "x"}, true); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range fake.calls {
		if call.Name == "podman" && len(call.Args) > 1 && call.Args[0] == "run" && call.Args[1] == "--rm" {
			found = true
			for _, f := range []string{"--memory", "--memory-swap", "--cpus", "--pids-limit"} {
				if _, ok := flagVal(call.Args, f); !ok {
					t.Errorf("probe container missing %s: %v", f, call.Args)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected the AVD-listing probe to run")
	}
}
