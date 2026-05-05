// Package vm provides QEMU full-system virtual machine orchestration
// for the vasic-digital container ecosystem. Sibling of pkg/emulator;
// shares pkg/cache for image artifacts.
//
// Constitutional landing: §6.K-debt criterion (2) — the QEMU baseline.
// API shape mirrors pkg/emulator (Boot/WaitForReady/Upload/Run/Download/
// Teardown + a MatrixRunner that emits the SAME attestation row schema).
//
// Anti-bluff posture (clauses 6.J/6.L inherited from Containers' parent):
// every public function has at least one falsifiability-rehearsed test.
// The hermetic unit tests in this directory drive QEMUVM via the
// processRunner / sshClient / qmpClient injection seams; production
// uses os/exec + golang.org/x/crypto/ssh + a real QMP socket. The
// real SSH/QMP impls are stubbed with explicit "not implemented in
// v0.1" errors — see clients.go — so any caller reaching them sees
// an honest signal. Real-impls land in a follow-up cycle; the
// operator's end-to-end matrix run is the v0.1 gate.
package vm

import (
	"context"
	"time"
)

// VMTarget identifies a single (arch, distro, version) tuple in the
// matrix. Matches an ImageEntry.ID in the project-side manifest.
type VMTarget struct {
	ID      string `json:"id"`     // matches ImageEntry.ID, e.g. "alpine-3.20-x86_64"
	Arch    string `json:"arch"`   // "x86_64" | "aarch64" | "riscv64"
	Distro  string `json:"distro"` // "alpine" | "debian" | "fedora"
	Version string `json:"version"`
}

// BootResult captures the outcome of a single QEMU launch.
type BootResult struct {
	Target        VMTarget
	Started       bool
	BootCompleted bool // true iff WaitForReady saw SSH up
	BootDuration  time.Duration
	SSHPort       int // host port forwarded to guest:22
	MonitorPort   int // host port for QMP control socket
	Error         error
}

// DiagnosticInfo is the per-VM forensic snapshot captured pre-script.
// Mirror of pkg/emulator.DiagnosticInfo; reviewer-facing per §6.I clause 4.
type DiagnosticInfo struct {
	Target    string `json:"target,omitempty"`     // VMTarget.ID
	Arch      string `json:"arch,omitempty"`       // observed (uname -m)
	Distro    string `json:"distro,omitempty"`     // observed (cat /etc/os-release | grep ^ID=)
	Kernel    string `json:"kernel,omitempty"`     // uname -r
	SSHBanner string `json:"ssh_banner,omitempty"` // sshd reply on connect
}

// FailureSummary is one captured failure from script stderr/exit-code.
// Same shape as pkg/emulator.FailureSummary so tag.sh's existing schema
// works unchanged for VM matrix attestations.
type FailureSummary struct {
	Class   string `json:"class,omitempty"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type"` // "exit-non-zero" | "stderr-pattern" | "<unparseable>"
	Message string `json:"message,omitempty"`
	Body    string `json:"body,omitempty"`
}

// VMConfig is the per-target configuration the VMMatrixRunner passes
// to runOne.
type VMConfig struct {
	Target        VMTarget
	QCowPath      string // path to read-only base image (from pkg/cache)
	Uploads       []UploadSpec
	Script        string // host-path to shell script run on guest
	Captures      []CaptureSpec
	BootTimeout   time.Duration
	ScriptTimeout time.Duration
	ColdBoot      bool // gating runs MUST be true (clause 6.I clause 6)
}

// UploadSpec is one file copied host→guest before script runs.
type UploadSpec struct {
	HostPath string `json:"host_path"`
	VMPath   string `json:"vm_path"`
}

// CaptureSpec is one file copied guest→host after script runs.
type CaptureSpec struct {
	VMPath      string `json:"vm_path"`
	HostSubpath string `json:"host_subpath"` // relative to evidence_dir/<target_id>/
}

// VMMatrixConfig drives a full matrix run.
type VMMatrixConfig struct {
	Targets       []VMTarget
	Uploads       []UploadSpec
	Script        string
	Captures      []CaptureSpec
	EvidenceDir   string
	BootTimeout   time.Duration // default per-arch from runner if zero
	ScriptTimeout time.Duration
	Concurrent    int    // default 1 (gating-eligible)
	Dev           bool   // permits snapshot reload; sets Gating=false
	ColdBoot      bool   // default true
	ImageManifest string // path to vm-images.json

	// NetworkProfile names a predefined network conditions profile.
	// Valid values: "edge", "2g", "3g", "4g", "lte", "wifi", "ethernet",
	// "none", or "" (no shaping). Phase 6 (Group C remaining): mirrors
	// the pkg/emulator profile set so a single matrix invocation can
	// shape both Android and VM rows under the same profile.
	NetworkProfile string

	// NetworkOverride supplies custom network conditions; any non-zero
	// field overrides the profile's value for that field.
	NetworkOverride NetworkConditions

	// CaptureScreenshotOnFailure enables forensic screenshot capture
	// when a per-VM row fails (Passed=false). Default: true. The VM
	// path uses QMP's `screendump` to a guest-side temp file, then
	// SCPs it down to the host.
	CaptureScreenshotOnFailure bool
}

// NetworkConditions parameterises a network-shaping profile for VMs.
// Mirror of pkg/emulator.NetworkConditions; declared independently here
// per the Decoupled Reusable Architecture rule (no cross-package coupling
// for a 4-field struct). Operators using both pkg/emulator and pkg/vm
// matrices should think of these as the same set of dimensions.
type NetworkConditions struct {
	DownKbps    int     `json:"down_kbps,omitempty"`
	UpKbps      int     `json:"up_kbps,omitempty"`
	LatencyMS   int     `json:"latency_ms,omitempty"`
	LossPercent float64 `json:"loss_percent,omitempty"`
}

// VMTestResult is the per-target row written to attestation.
type VMTestResult struct {
	Target           VMTarget         `json:"target"`
	Started          time.Time        `json:"started_at"`
	Duration         time.Duration    `json:"duration"`
	BootSeconds      float64          `json:"boot_seconds"`
	BootError        string           `json:"boot_error,omitempty"`
	ScriptExitCode   int              `json:"script_exit_code"`
	ScriptStderr     string           `json:"script_stderr,omitempty"`
	Passed           bool             `json:"passed"`
	Diag             DiagnosticInfo   `json:"diag"`
	FailureSummaries []FailureSummary `json:"failure_summaries"`
	Concurrent       int              `json:"concurrent"`
	CapturedFiles    []string         `json:"captured_files,omitempty"`
	// NetworkProfile is the active --network-profile name at the time
	// this row ran, or "" for no shaping. Phase 6 (Group C remaining).
	NetworkProfile string `json:"network_profile,omitempty"`
	// ScreenshotPath is the path (relative to EvidenceDir) of the
	// forensic guest-screen capture written when Passed=false AND the
	// matrix's CaptureScreenshotOnFailure flag is true. Empty when
	// either condition is false OR the QMP screendump failed.
	ScreenshotPath string `json:"screenshot_path,omitempty"`
}

// VMMatrixResult is the matrix-level aggregate.
type VMMatrixResult struct {
	Config          VMMatrixConfig
	Rows            []VMTestResult
	StartedAt       time.Time
	FinishedAt      time.Time
	AttestationFile string
	Gating          bool // true ⇔ Concurrent==1 AND !Dev
}

// AllPassed returns true iff every row's Passed is true.
func (r VMMatrixResult) AllPassed() bool {
	for _, row := range r.Rows {
		if !row.Passed {
			return false
		}
	}
	return true
}

// VM is the per-target contract a target-specific VM implementation
// satisfies. Production: QEMUVM.
type VM interface {
	Boot(ctx context.Context, config VMConfig) (BootResult, error)
	WaitForReady(ctx context.Context, sshPort int, timeout time.Duration) error
	Upload(ctx context.Context, sshPort int, hostPath, vmPath string) error
	Run(ctx context.Context, sshPort int, script string, env map[string]string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
	Download(ctx context.Context, sshPort int, vmPath, hostPath string) error
	Teardown(ctx context.Context, monitorPort, sshPort int) error
}

// VMMatrixRunner orchestrates a sequence of (VMTarget, script) pairs.
type VMMatrixRunner interface {
	RunMatrix(ctx context.Context, config VMMatrixConfig) (VMMatrixResult, error)
}

// KillReport summarises the outcome of a KillByQEMUMonitorPort invocation.
//
// Same shape as pkg/emulator.KillReport — declared here in pkg/vm so the
// matrix runner's Teardown fast-path doesn't pull in pkg/emulator just
// for a 4-field struct. The Matched count is the gate the caller (Teardown
// fast-path) uses to decide whether the kill succeeded enough to short-
// circuit the "vm did not exit" error path. Matched=0 is a no-op safe
// state: it means no /proc entry passed the strict adjacent-token check,
// and the caller MUST treat that as "fast-path skipped" — typically by
// returning the original timeout error so the matrix runner records an
// honest row failure.
type KillReport struct {
	// Matched is the number of /proc entries whose argv contained the
	// adjacent pair `-monitor`, `tcp:127.0.0.1:<port>,server,nowait`.
	Matched int
	// Sigtermed lists the PIDs that received SIGTERM (every matched
	// PID receives one).
	Sigtermed []int
	// Sigkilled lists the PIDs that survived the 5-second SIGTERM
	// grace and therefore received SIGKILL.
	Sigkilled []int
	// Surviving lists PIDs still alive after SIGKILL (rare; caused
	// by permission errors, kernel-level holds, or PID-reuse races).
	Surviving []int
}
