package hostbudget

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// G2 (CAT-DEVEMU) PROVISIONAL: fake-/proc fixtures. The unit tests prove the
// verdict logic; they do NOT prove a real host's numbers (one real-host smoke
// test at the bottom reads the live /proc, never boots anything).

const gib = int64(1) << 30

type fx struct {
	memTotalKB, memAvailKB int64
	load1                  string
	memSome, memFull       string // "avg10" values; "" => file absent/unreadable
	cpuSome                string
	procs                  int // processes owned by uid 1000, 1 thread each unless ownThreads set
	ownThreads             int
	otherProcs             int // processes owned by uid 0
	otherThreads           int
}

func writeProc(t *testing.T, f fx) string {
	t.Helper()
	root := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	w("meminfo", "MemTotal:       "+itoa(f.memTotalKB)+" kB\nMemFree:  1 kB\nMemAvailable:   "+itoa(f.memAvailKB)+" kB\n")
	w("loadavg", f.load1+" 1.0 1.0 1/100 1\n")
	if f.memSome != "" {
		w("pressure/memory", "some avg10="+f.memSome+" avg60=0.00 avg300=0.00 total=1\nfull avg10="+f.memFull+" avg60=0.00 avg300=0.00 total=1\n")
	}
	if f.cpuSome != "" {
		w("pressure/cpu", "some avg10="+f.cpuSome+" avg60=0.00 avg300=0.00 total=1\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n")
	}
	thr := f.ownThreads
	if thr == 0 {
		thr = 1
	}
	pid := 1
	for i := 0; i < f.procs; i++ {
		w(itoa(int64(pid))+"/status", "Name:\tx\nUid:\t1000\t1000\t1000\t1000\nThreads:\t"+itoa(int64(thr))+"\n")
		pid++
	}
	for i := 0; i < f.otherProcs; i++ {
		w(itoa(int64(pid))+"/status", "Name:\ty\nUid:\t0\t0\t0\t0\nThreads:\t"+itoa(int64(f.otherThreads))+"\n")
		pid++
	}
	return root
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func goodFx() fx {
	return fx{memTotalKB: 30 * 1024 * 1024, memAvailKB: 17 * 1024 * 1024, load1: "2.00",
		memSome: "1.08", memFull: "0.00", cpuSome: "5.00", procs: 10}
}

func probe(root string) Probe {
	return Probe{ProcRoot: root, NumCPU: func() int { return 16 }, KVMPath: "", UID: func() int { return 1000 },
		PidsLimit: func() (uint64, bool, error) { return 100000, true, nil }}
}

func TestAdmit_GoldenGood_Fits(t *testing.T) {
	ev := Admit(Need{MemBytes: 5 * gib, CPUs: 3}, DefaultPolicy(), probe(writeProc(t, goodFx())))
	assert.Equal(t, Fits, ev.Verdict, "%+v", ev.Reasons)
	assert.Empty(t, ev.Reasons)
	assert.NotZero(t, ev.Inputs.MemTotalBytes)
}

func TestAdmit_GoldenBad_MemoryDoesNotFit(t *testing.T) {
	f := goodFx()
	f.memAvailKB = 6 * 1024 * 1024 // 6 GiB avail, need 5 => 1 GiB left < reserve
	ev := Admit(Need{MemBytes: 5 * gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict)
	assert.Contains(t, strings.Join(ev.Reasons, "|"), "reserve")
}

func TestAdmit_SixtyPercentCeiling(t *testing.T) {
	// 30 GiB total, 5 GiB available => 25 GiB in use (83%). Even a tiny need
	// sits above the 60% ceiling (18 GiB) => does_not_fit via the ceiling rule.
	f := goodFx()
	f.memAvailKB = 5 * 1024 * 1024
	ev := Admit(Need{MemBytes: 1}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict)
	assert.Contains(t, strings.Join(ev.Reasons, "|"), "60%")
}

func TestAdmit_CeilingBoundary(t *testing.T) {
	// total 30 GiB: ceiling 18 GiB. used 12 GiB + need 6 GiB = exactly 18 => fits
	// (if reserve also ok); need 6 GiB + 1 byte => does_not_fit.
	f := goodFx()
	f.memAvailKB = 18 * 1024 * 1024
	root := writeProc(t, f)
	assert.Equal(t, Fits, Admit(Need{MemBytes: 6 * gib}, DefaultPolicy(), probe(root)).Verdict)
	ev := Admit(Need{MemBytes: 6*gib + 1}, DefaultPolicy(), probe(root))
	assert.Equal(t, DoesNotFit, ev.Verdict)
}

func TestAdmit_MemoryPressureRefuses(t *testing.T) {
	f := goodFx()
	f.memSome = "40.00"
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict)
	f = goodFx()
	f.memFull = "3.50"
	ev = Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict, "full avg10 above threshold must refuse")
}

func TestAdmit_UnreadablePSI_IsUndeterminedNeverFits(t *testing.T) {
	f := goodFx()
	f.memSome = "" // pressure/memory absent
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, Undetermined, ev.Verdict)
	f = goodFx()
	f.cpuSome = ""
	ev = Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, Undetermined, ev.Verdict)
}

func TestAdmit_UnparsableMeminfo_IsUndetermined(t *testing.T) {
	root := writeProc(t, goodFx())
	require.NoError(t, os.WriteFile(filepath.Join(root, "meminfo"), []byte("garbage\n"), 0o644))
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(root))
	assert.Equal(t, Undetermined, ev.Verdict)
}

func TestAdmit_HighLoadRefusesEvenWhenMemoryFits_BothAxesReported(t *testing.T) {
	f := goodFx()
	f.load1 = "26.4" // 16 cpus * 0.75 = 12
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict)
	assert.Contains(t, strings.Join(ev.Reasons, "|"), "load")
	// carrier: high load AND low memory -> both axes named
	f.memAvailKB = 3 * 1024 * 1024
	ev = Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	joined := strings.Join(ev.Reasons, "|")
	assert.Contains(t, joined, "load")
	assert.Contains(t, joined, "reserve")
}

func TestAdmit_CPUPressureRefuses(t *testing.T) {
	f := goodFx()
	f.cpuSome = "80.00"
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict)
}

func TestAdmit_Pids(t *testing.T) {
	root := writeProc(t, goodFx()) // 10 processes
	p := probe(root)
	p.PidsLimit = func() (uint64, bool, error) { return 50, true, nil }
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: gib, Pids: 100}, DefaultPolicy(), p).Verdict)
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib, Pids: 30}, DefaultPolicy(), p).Verdict)
	p.PidsLimit = func() (uint64, bool, error) { return 0, false, os.ErrPermission }
	assert.Equal(t, Undetermined, Admit(Need{MemBytes: gib, Pids: 30}, DefaultPolicy(), p).Verdict)
	// unlimited
	p.PidsLimit = func() (uint64, bool, error) { return 0, false, nil }
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib, Pids: 1 << 20}, DefaultPolicy(), p).Verdict)
}

func TestKVMUsable_OpenForWriteNotStat(t *testing.T) {
	d := t.TempDir()
	// exists but read-only: stat succeeds, open-for-write fails (running as non-root)
	if os.Geteuid() == 0 {
		t.Skip("SKIP-OK: root can open read-only files for write; probe meaningful only for non-root")
	}
	ro := filepath.Join(d, "kvm-ro")
	require.NoError(t, os.WriteFile(ro, nil, 0o444))
	_, statErr := os.Stat(ro)
	require.NoError(t, statErr)
	assert.Error(t, KVMUsable(ro), "a stat-only check would have said OK; open-for-write must refuse")
	rw := filepath.Join(d, "kvm-rw")
	require.NoError(t, os.WriteFile(rw, nil, 0o644))
	assert.NoError(t, KVMUsable(rw))
	assert.Error(t, KVMUsable(filepath.Join(d, "missing")))
}

func TestAdmit_KVM(t *testing.T) {
	d := t.TempDir()
	if os.Geteuid() == 0 {
		t.Skip("SKIP-OK: root bypasses file modes")
	}
	ro := filepath.Join(d, "kvm-ro")
	require.NoError(t, os.WriteFile(ro, nil, 0o444))
	p := probe(writeProc(t, goodFx()))
	p.KVMPath = ro
	ev := Admit(Need{MemBytes: gib, NeedsKVM: true}, DefaultPolicy(), p)
	assert.Equal(t, DoesNotFit, ev.Verdict)
	assert.Contains(t, strings.Join(ev.Reasons, "|"), "kvm")
	// not needed => not probed
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib}, DefaultPolicy(), p).Verdict)
}

func TestAdmit_UndeterminedNeverUpgradedToFits_AndDoesNotFitWins(t *testing.T) {
	f := goodFx()
	f.memSome = "" // undetermined axis
	f.load1 = "30" // definitive does_not_fit axis
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f)))
	assert.Equal(t, DoesNotFit, ev.Verdict)
}

func TestEvidence_JSONRoundTripAndWrite(t *testing.T) {
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, goodFx())))
	dir := t.TempDir()
	path, err := ev.WriteJSON(dir, "unit")
	require.NoError(t, err)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var back Evidence
	require.NoError(t, json.Unmarshal(b, &back))
	assert.Equal(t, ev.Verdict, back.Verdict)
	assert.Equal(t, ev.Inputs.MemAvailableBytes, back.Inputs.MemAvailableBytes)
	_, err = ev.WriteJSON(dir, "../escape")
	assert.Error(t, err, "label with path separators must be refused")
}

func TestDefaultPolicy_Values(t *testing.T) {
	p := DefaultPolicy()
	assert.Equal(t, 60, p.MemCeilingPct)
	assert.Equal(t, 4*gib, p.ReserveMinBytes)
	assert.Equal(t, 15, p.ReservePct)
}

// Real host smoke: reads live /proc only. Asserts structural sanity, never a
// verdict (the verdict depends on the host's load at run time).
func TestAdmit_RealHostSmoke(t *testing.T) {
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		t.Skip("SKIP-OK: no /proc")
	}
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), Probe{})
	assert.Contains(t, []Verdict{Fits, DoesNotFit, Undetermined}, ev.Verdict)
	assert.Greater(t, ev.Inputs.MemTotalBytes, int64(0))
}

// ---- round-2 review fixes ----

func TestR2_F7_PidsCountTasksOfCurrentUIDOnly(t *testing.T) {
	f := goodFx()
	f.procs, f.ownThreads = 10, 1         // 10 own tasks
	f.otherProcs, f.otherThreads = 500, 4 // 2000 tasks of root: must NOT count
	p := probe(writeProc(t, f))
	p.PidsLimit = func() (uint64, bool, error) { return 50, true, nil }
	ev := Admit(Need{MemBytes: gib, Pids: 30}, DefaultPolicy(), p)
	assert.Equal(t, Fits, ev.Verdict, "%v", ev.Reasons)
	assert.Equal(t, 10, ev.Inputs.ProcCount)
}

func TestR2_F7_ThreadsCountAsTasks(t *testing.T) {
	f := goodFx()
	f.procs, f.ownThreads = 5, 8 // 40 tasks
	p := probe(writeProc(t, f))
	p.PidsLimit = func() (uint64, bool, error) { return 50, true, nil }
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: gib, Pids: 20}, DefaultPolicy(), p).Verdict)
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib, Pids: 10}, DefaultPolicy(), p).Verdict)
}

func TestR2_F7_PidsExactBoundary(t *testing.T) {
	p := probe(writeProc(t, goodFx())) // 10 own tasks
	p.PidsLimit = func() (uint64, bool, error) { return 50, true, nil }
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib, Pids: 40}, DefaultPolicy(), p).Verdict, "limit-running == need fits")
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: gib, Pids: 41}, DefaultPolicy(), p).Verdict)
}

func TestR2_F7_UnparsableProcessTableIsUndetermined(t *testing.T) {
	f := goodFx()
	root := writeProc(t, f)
	// every pid dir loses its status file
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		if e.IsDir() && e.Name() != "pressure" {
			_ = os.Remove(filepath.Join(root, e.Name(), "status"))
		}
	}
	p := probe(root)
	p.PidsLimit = func() (uint64, bool, error) { return 50, true, nil }
	assert.Equal(t, Undetermined, Admit(Need{MemBytes: gib, Pids: 5}, DefaultPolicy(), p).Verdict)
}

func TestR2_F11_ReserveFloorBinds(t *testing.T) {
	// total 10 GiB: 15% = 1.5 GiB < floor 4 GiB, so the floor is the reserve.
	f := fx{memTotalKB: 10 * 1024 * 1024, memAvailKB: 8 * 1024 * 1024, load1: "1", memSome: "0", memFull: "0", cpuSome: "0", procs: 1}
	root := writeProc(t, f)
	assert.Equal(t, Fits, Admit(Need{MemBytes: 4 * gib}, DefaultPolicy(), probe(root)).Verdict, "left exactly 4 GiB == floor")
	ev := Admit(Need{MemBytes: 4*gib + 1}, DefaultPolicy(), probe(root))
	assert.Equal(t, DoesNotFit, ev.Verdict)
	assert.Contains(t, strings.Join(ev.Reasons, "|"), "reserve")
}

func TestR2_F11_ReservePercentBindsWithLooserCeiling(t *testing.T) {
	pol := DefaultPolicy()
	pol.MemCeilingPct = 95
	f := fx{memTotalKB: 100 * 1024 * 1024, memAvailKB: 20 * 1024 * 1024, load1: "1", memSome: "0", memFull: "0", cpuSome: "0", procs: 1}
	root := writeProc(t, f)
	assert.Equal(t, Fits, Admit(Need{MemBytes: 5 * gib}, pol, probe(root)).Verdict, "left 15 GiB == 15%")
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: 6 * gib}, pol, probe(root)).Verdict)
}

func TestR2_F11_PSIAndLoadBoundaries(t *testing.T) {
	f := goodFx()
	f.memSome = "10.00"
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f))).Verdict, "== limit is not over it")
	f.memSome = "10.01"
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f))).Verdict)
	f = goodFx()
	f.memFull = "2.00"
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f))).Verdict, "full == limit")
	f.memFull = "2.01"
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f))).Verdict)
	f = goodFx()
	f.load1 = "12.00" // 16 * 0.75
	assert.Equal(t, Fits, Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f))).Verdict)
	f.load1 = "12.01"
	assert.Equal(t, DoesNotFit, Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, f))).Verdict)
}

func TestR2_F12_NegativeNeedIsRefusedNotFits(t *testing.T) {
	root := writeProc(t, goodFx())
	for name, n := range map[string]Need{
		"neg mem":  {MemBytes: -1},
		"neg cpus": {MemBytes: gib, CPUs: -1},
		"neg pids": {MemBytes: gib, Pids: -5},
	} {
		ev := Admit(n, DefaultPolicy(), probe(root))
		assert.Equal(t, Undetermined, ev.Verdict, name)
		assert.Contains(t, strings.Join(ev.Reasons, "|"), "invalid need", name)
	}
}

func TestR2_F12_WriteJSONDoesNotFollowSymlinkOrOverwrite(t *testing.T) {
	ev := Admit(Need{MemBytes: gib}, DefaultPolicy(), probe(writeProc(t, goodFx())))
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(victim, filepath.Join(dir, "hostbudget-x.json")))
	_, err := ev.WriteJSON(dir, "x")
	assert.Error(t, err, "an existing path (symlink) must be refused, not followed")
	b, _ := os.ReadFile(victim)
	assert.Equal(t, "keep", string(b))
	// existing regular file is not overwritten either
	_, err = ev.WriteJSON(dir, "y")
	require.NoError(t, err)
	_, err = ev.WriteJSON(dir, "y")
	assert.Error(t, err)
}
