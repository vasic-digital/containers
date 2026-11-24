// Package hostbudget answers one question before a heavy workload (an
// emulator, a VM, a build container) is started: does the host's LIVE state
// leave room for it? The verdict vocabulary is closed — fits, does_not_fit,
// undetermined — and undetermined is never read as fits.
//
// It reads /proc only (meminfo, loadavg, pressure/{memory,cpu}, the process
// table) plus an open-for-write probe of the KVM device. It starts nothing,
// signals nothing and writes nothing except the optional evidence file.
//
// Defaults follow the constitution's host-safety rules: the §12.6 60% memory
// ceiling, a free-memory reserve of max(4 GiB, 15% of MemTotal), the §11.4.277
// pressure limits (memory PSI some avg10 > 10 or full avg10 > 2) and a load
// limit of 0.75 per CPU. Every threshold is consumer DATA via Policy.
//
// PROVISIONAL (CAT-DEVEMU G2): fixture-tested; the threshold values are
// starting points and have not been calibrated against a long real-host run.
package hostbudget

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Verdict is the closed result set.
type Verdict string

const (
	Fits         Verdict = "fits"
	DoesNotFit   Verdict = "does_not_fit"
	Undetermined Verdict = "undetermined"
)

// Need is what the workload will consume.
type Need struct {
	MemBytes int64   `json:"mem_bytes"`
	CPUs     float64 `json:"cpus"`
	Pids     int64   `json:"pids"`
	NeedsKVM bool    `json:"needs_kvm"`
}

// Policy holds the thresholds (consumer DATA).
type Policy struct {
	MemCeilingPct   int     `json:"mem_ceiling_pct"`
	ReserveMinBytes int64   `json:"reserve_min_bytes"`
	ReservePct      int     `json:"reserve_pct"`
	MaxLoadPerCPU   float64 `json:"max_load_per_cpu"`
	MemSomeAvg10Max float64 `json:"mem_some_avg10_max"`
	MemFullAvg10Max float64 `json:"mem_full_avg10_max"`
	CPUSomeAvg10Max float64 `json:"cpu_some_avg10_max"`
}

// DefaultPolicy returns the constitution-derived defaults.
func DefaultPolicy() Policy {
	return Policy{
		MemCeilingPct:   60,
		ReserveMinBytes: 4 << 30,
		ReservePct:      15,
		MaxLoadPerCPU:   0.75,
		MemSomeAvg10Max: 10,
		MemFullAvg10Max: 2,
		CPUSomeAvg10Max: 50,
	}
}

// Probe holds the injectable seams; the zero value probes the live host.
type Probe struct {
	// ProcRoot defaults to /proc.
	ProcRoot string
	// NumCPU defaults to runtime.NumCPU.
	NumCPU func() int
	// KVMPath, when set and Need.NeedsKVM, is opened O_RDWR.
	KVMPath string
	// UID is the user whose tasks count against RLIMIT_NPROC (default os.Getuid).
	UID func() int
	// PidsLimit returns RLIMIT_NPROC (limit, limited=false means unlimited).
	PidsLimit func() (uint64, bool, error)
}

// Inputs records what was actually read.
type Inputs struct {
	MemTotalBytes     int64   `json:"mem_total_bytes"`
	MemAvailableBytes int64   `json:"mem_available_bytes"`
	Load1             float64 `json:"load1"`
	NumCPU            int     `json:"num_cpu"`
	MemSomeAvg10      float64 `json:"mem_some_avg10"`
	MemFullAvg10      float64 `json:"mem_full_avg10"`
	CPUSomeAvg10      float64 `json:"cpu_some_avg10"`
	ProcCount         int     `json:"proc_count"`
	PidsLimit         string  `json:"pids_limit,omitempty"`
	KVM               string  `json:"kvm,omitempty"`
}

// Evidence is the full, serialisable result.
type Evidence struct {
	Verdict Verdict  `json:"verdict"`
	Reasons []string `json:"reasons"`
	Need    Need     `json:"need"`
	Policy  Policy   `json:"policy"`
	Inputs  Inputs   `json:"inputs"`
}

// KVMUsable proves the device can really be used: it opens it for writing and
// closes it (no ioctl). A stat is NOT enough — ACL-only access makes the node
// exist but not be openable.
func KVMUsable(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return f.Close()
}

// Admit evaluates the host against the need. Any definitive refusal wins over
// an undetermined axis; undetermined wins over fits.
func Admit(need Need, pol Policy, pr Probe) Evidence {
	ev := Evidence{Need: need, Policy: pol}
	if need.MemBytes < 0 || !(need.CPUs >= 0) || math.IsInf(need.CPUs, 0) || need.Pids < 0 {
		ev.Verdict = Undetermined
		ev.Reasons = []string{fmt.Sprintf("invalid need (negative, NaN or infinite): mem=%d cpus=%g pids=%d", need.MemBytes, need.CPUs, need.Pids)}
		if math.IsNaN(need.CPUs) || math.IsInf(need.CPUs, 0) {
			ev.Need.CPUs = 0 // JSON cannot carry NaN/Inf; the raw value is in Reasons
		}
		return ev
	}
	var refuse, unknown []string
	root := pr.ProcRoot
	if root == "" {
		root = "/proc"
	}
	ncpu := runtime.NumCPU
	if pr.NumCPU != nil {
		ncpu = pr.NumCPU
	}
	ev.Inputs.NumCPU = ncpu()

	// memory
	total, avail, err := readMeminfo(filepath.Join(root, "meminfo"))
	if err != nil {
		unknown = append(unknown, "meminfo unreadable: "+err.Error())
	} else {
		ev.Inputs.MemTotalBytes, ev.Inputs.MemAvailableBytes = total, avail
		used := total - avail
		ceiling := total * int64(pol.MemCeilingPct) / 100
		if used+need.MemBytes > ceiling {
			refuse = append(refuse, fmt.Sprintf("memory: in use %d + need %d exceeds the %d%% ceiling %d (§12.6)",
				used, need.MemBytes, pol.MemCeilingPct, ceiling))
		}
		reserve := total * int64(pol.ReservePct) / 100
		if reserve < pol.ReserveMinBytes {
			reserve = pol.ReserveMinBytes
		}
		if avail-need.MemBytes < reserve {
			refuse = append(refuse, fmt.Sprintf("memory: available %d - need %d is below the reserve %d",
				avail, need.MemBytes, reserve))
		}
	}

	// load
	load1, err := readLoad1(filepath.Join(root, "loadavg"))
	if err != nil {
		unknown = append(unknown, "loadavg unreadable: "+err.Error())
	} else {
		ev.Inputs.Load1 = load1
		if limit := float64(ev.Inputs.NumCPU) * pol.MaxLoadPerCPU; load1 > limit {
			refuse = append(refuse, fmt.Sprintf("load: 1-minute load %.2f exceeds %.2f (%d cpus x %.2f)",
				load1, limit, ev.Inputs.NumCPU, pol.MaxLoadPerCPU))
		}
	}

	// pressure
	if some, full, err := readPSI(filepath.Join(root, "pressure", "memory")); err != nil {
		unknown = append(unknown, "memory PSI unreadable: "+err.Error())
	} else {
		ev.Inputs.MemSomeAvg10, ev.Inputs.MemFullAvg10 = some, full
		if some > pol.MemSomeAvg10Max {
			refuse = append(refuse, fmt.Sprintf("pressure: memory some avg10 %.2f > %.2f", some, pol.MemSomeAvg10Max))
		}
		if full > pol.MemFullAvg10Max {
			refuse = append(refuse, fmt.Sprintf("pressure: memory full avg10 %.2f > %.2f", full, pol.MemFullAvg10Max))
		}
	}
	if some, _, err := readPSI(filepath.Join(root, "pressure", "cpu")); err != nil {
		unknown = append(unknown, "cpu PSI unreadable: "+err.Error())
	} else {
		ev.Inputs.CPUSomeAvg10 = some
		if some > pol.CPUSomeAvg10Max {
			refuse = append(refuse, fmt.Sprintf("pressure: cpu some avg10 %.2f > %.2f", some, pol.CPUSomeAvg10Max))
		}
	}

	// pids
	if need.Pids > 0 {
		limFn := pr.PidsLimit
		if limFn == nil {
			limFn = rlimitNproc
		}
		lim, limited, err := limFn()
		uidFn := pr.UID
		if uidFn == nil {
			uidFn = os.Getuid
		}
		n, cerr := countTasks(root, uidFn())
		switch {
		case err != nil:
			unknown = append(unknown, "RLIMIT_NPROC unreadable: "+err.Error())
		case cerr != nil:
			unknown = append(unknown, "task table unreadable: "+cerr.Error())
		default:
			ev.Inputs.ProcCount = n
			if !limited {
				ev.Inputs.PidsLimit = "unlimited"
			} else {
				ev.Inputs.PidsLimit = strconv.FormatUint(lim, 10)
				if int64(lim)-int64(n) < need.Pids {
					refuse = append(refuse, fmt.Sprintf("pids: limit %d - running %d is below need %d", lim, n, need.Pids))
				}
			}
		}
	}

	// kvm
	if need.NeedsKVM {
		if pr.KVMPath == "" {
			unknown = append(unknown, "kvm required but no KVMPath given")
		} else if err := KVMUsable(pr.KVMPath); err != nil {
			ev.Inputs.KVM = "not usable"
			refuse = append(refuse, "kvm: "+pr.KVMPath+" cannot be opened read-write: "+err.Error())
		} else {
			ev.Inputs.KVM = "usable"
		}
	}

	switch {
	case len(refuse) > 0:
		ev.Verdict = DoesNotFit
	case len(unknown) > 0:
		ev.Verdict = Undetermined
	default:
		ev.Verdict = Fits
	}
	ev.Reasons = append(append([]string{}, refuse...), unknown...)
	return ev
}

var labelRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// syncFile is a seam so tests can inject an fsync failure.
var syncFile = func(f *os.File) error { return f.Sync() }

// WriteJSON stores the evidence as <dir>/hostbudget-<label>.json (0600).
func (e Evidence) WriteJSON(dir, label string) (string, error) {
	if !labelRe.MatchString(label) || strings.Contains(label, "..") {
		return "", fmt.Errorf("hostbudget: unsafe evidence label %q", label)
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "hostbudget-"+label+".json")
	// O_EXCL: an existing path (including a symlink) is refused, never followed
	// or overwritten.
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	// We created the file (O_EXCL), so we own it and may remove a partial one.
	fail := func(err error) (string, error) {
		_ = f.Close()
		_ = os.Remove(p)
		return "", err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fail(err)
	}
	if err := syncFile(f); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(p)
		return "", err
	}
	return p, nil
}

func readMeminfo(path string) (total, avail int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	var gotT, gotA bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 2 {
			continue
		}
		v, perr := strconv.ParseInt(fs[1], 10, 64)
		if perr != nil {
			continue
		}
		switch fs[0] {
		case "MemTotal:":
			total, gotT = v*1024, true
		case "MemAvailable:":
			avail, gotA = v*1024, true
		}
	}
	if !gotT || !gotA || total <= 0 {
		return 0, 0, errors.New("MemTotal/MemAvailable not found")
	}
	return total, avail, nil
}

func readLoad1(path string) (float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fs := strings.Fields(string(b))
	if len(fs) < 1 {
		return 0, errors.New("empty loadavg")
	}
	return strconv.ParseFloat(fs[0], 64)
}

// readPSI returns the "some" and "full" avg10 values; "full" is 0 when absent
// (the cpu file's full line is not meaningful on every kernel).
func readPSI(path string) (some, full float64, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var gotSome bool
	for _, line := range strings.Split(string(b), "\n") {
		fs := strings.Fields(line)
		if len(fs) < 2 {
			continue
		}
		for _, f := range fs[1:] {
			if v, ok := strings.CutPrefix(f, "avg10="); ok {
				x, perr := strconv.ParseFloat(v, 64)
				if perr != nil {
					return 0, 0, perr
				}
				switch fs[0] {
				case "some":
					some, gotSome = x, true
				case "full":
					full = x
				}
			}
		}
	}
	if !gotSome {
		return 0, 0, errors.New("no 'some avg10' line")
	}
	return some, full, nil
}

// countTasks sums the Threads of every process whose real UID is uid — the
// quantity RLIMIT_NPROC limits (tasks of one user, threads included). A process
// table with entries but no readable status is an error (never "0 tasks").
// readStatus is a seam so tests can inject read errors.
var readStatus = os.ReadFile

func countTasks(root string, uid int) (int, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	total, dirs, parsed := 0, 0, 0
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		dirs++
		b, err := readStatus(filepath.Join(root, e.Name(), "status"))
		if err != nil {
			// ENOENT/ESRCH: the process exited between ReadDir and the read.
			// EACCES (for example hidepid mounts) is NOT skipped: it makes the
			// count unknown.
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
				continue // the process exited between ReadDir and ReadFile
			}
			// unreadable for any other reason: the count is unknown, and a
			// partial count would under-report (R2-9)
			return 0, fmt.Errorf("pid %s status unreadable: %w", e.Name(), err)
		}
		puid, threads, ok := parseStatus(string(b))
		if !ok {
			return 0, fmt.Errorf("pid %s status has no Uid/Threads", e.Name())
		}
		parsed++
		if puid == uid {
			total += threads
		}
	}
	if dirs > 0 && parsed == 0 {
		return 0, errors.New("no process status could be parsed")
	}
	return total, nil
}

func parseStatus(s string) (uid, threads int, ok bool) {
	var gotU, gotT bool
	for _, l := range strings.Split(s, "\n") {
		fs := strings.Fields(l)
		if len(fs) < 2 {
			continue
		}
		switch fs[0] {
		case "Uid:":
			if v, err := strconv.Atoi(fs[1]); err == nil {
				uid, gotU = v, true
			}
		case "Threads:":
			if v, err := strconv.Atoi(fs[1]); err == nil {
				threads, gotT = v, true
			}
		}
	}
	return uid, threads, gotU && gotT
}
