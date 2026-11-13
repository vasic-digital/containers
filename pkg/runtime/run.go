package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrRunUnsupported is returned by ContainerRuntime.Run implementations whose
// underlying tool has no one-shot ephemeral-run form matching Run's contract.
// It is a SENTINEL, not a silent no-op: a caller can distinguish "this runtime
// cannot do it" from "it ran and failed" with errors.Is, and neither case is
// ever reported as success. Wrapped by crictl, LXD, Kubernetes and the remote
// SSH proxy.
var ErrRunUnsupported = errors.New("ephemeral run is not supported by this runtime")

// ErrStdinUnsupported is returned when a caller supplies WithRunStdin but the
// runtime's injected CommandExecutor cannot stream stdin (it does not implement
// StdinExecutor).
//
// This is deliberately an ERROR and not a degradation to "run it without
// stdin". The whole point of the primitive is streaming a document into the
// container; running the same command with an empty stdin would produce a
// plausible-looking empty result and a zero exit code, which is precisely the
// silent-failure shape the log reader in this package was just fixed for.
var ErrStdinUnsupported = errors.New(
	"executor does not support stdin: implement runtime.StdinExecutor",
)

// StdinExecutor is the OPTIONAL stdin-capable half of CommandExecutor.
//
// It is a separate interface rather than three more methods on CommandExecutor
// because CommandExecutor is exported and adding to it would break every
// external implementer (§11.4.122). defaultExecutor implements it, so the
// production path always has stdin; an injected fake that does not gets
// ErrStdinUnsupported the moment stdin is actually requested — never a
// silently discarded document.
type StdinExecutor interface {
	// ExecuteWithStdin runs a command with stdin streamed from r (nil means no
	// stdin) and returns its stdout, stderr and exit code. As with
	// ExecuteWithStderr, a non-zero exit is reported through the exit code with
	// a nil error; the error is reserved for failures to run the command at all.
	ExecuteWithStdin(
		ctx context.Context, r io.Reader, name string, args ...string,
	) ([]byte, []byte, int, error)
}

// ExecuteWithStdin implements StdinExecutor for the real os/exec path.
func (e *defaultExecutor) ExecuteWithStdin(
	ctx context.Context, r io.Reader, name string, args ...string,
) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 2 * time.Second
	if r != nil {
		cmd.Stdin = r
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
		err = nil
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode, err
}

// --- Run Options ---

type runOptions struct {
	Stdin      io.Reader
	Remove     bool
	Name       string
	Entrypoint string
	WorkDir    string
	User       string
	Network    string
	Env        map[string]string
	Volumes    []string
	ExtraArgs  []string

	// Allowlist-design options (round 3). Mounts, capabilities and
	// security options exist ONLY as typed options; free-form ExtraArgs may
	// carry only the flags in extraFlagAllowlist.
	Mounts       []RunMount
	MountRoots   []string
	CapAdd       []string
	CapReasons   []string
	CapDropAll   bool
	NoNewPrivs   bool
	remote       bool // no local filesystem inspection (set by ResolveRunSpec)
	resolvedMnts []string

	// Typed host-safety options (G1, CAT-DEVEMU). Raw values are recorded by
	// the With* options and validated together in validateRunOptions, so a
	// bad value can never reach an argv.
	Memory           int64
	memorySet        bool
	MemorySwap       int64
	memorySwapSet    bool
	CPUs             float64
	cpusSet          bool
	PidsLimit        int64
	pidsSet          bool
	Devices          []runDevice
	Userns           string
	Publishes        []runPublish
	PublishAllIfaces bool
	Labels           map[string]string
	RequireDigest    bool

	// runtimeName is set by runViaCLI ("podman", "docker", "nerdctl"); empty
	// for the runtime-agnostic ResolveRunSpec.
	runtimeName string
}

type runDevice struct {
	Path  string
	Allow []string
}

type runPublish struct {
	HostIP   string
	HostPort int
	CtrPort  int
	Proto    string
}

// Typed errors of the run builder. All are sentinels for errors.Is.
var (
	// ErrRunForbiddenOption: the request asks for a host-escalating form
	// (--privileged, host namespaces, ALL capabilities, unconfined security
	// options, an image reference that parses as a flag). Never overridable.
	ErrRunForbiddenOption = errors.New("run option is forbidden (host-escalating)")
	// ErrRunUnsupportedOption: the option exists but the selected runtime
	// cannot honour it (for example --userns=keep-id on docker).
	ErrRunUnsupportedOption = errors.New("run option is not supported by this runtime")
	// ErrRunInvalidOption: a typed option carries an out-of-range or
	// malformed value.
	ErrRunInvalidOption = errors.New("run option has an invalid value")
)

// deviceUsable is the seam that proves a device can really be used. For
// /dev/kvm it opens the node O_RDWR (a stat does not prove usability: ACL-only
// access, see pkg/emulator). Package var so tests can inject.
var deviceUsable = func(p string) error {
	if p == "/dev/kvm" {
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		return f.Close()
	}
	_, err := os.Stat(p)
	return err
}

var digestRef = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)

// WithRunMemory caps container memory in bytes (--memory). Swap defaults to the
// same value (no swap) unless WithRunMemorySwap is given.
func WithRunMemory(bytes int64) RunOption {
	return func(o *runOptions) { o.Memory, o.memorySet = bytes, true }
}

// WithRunMemorySwap sets --memory-swap (total memory+swap) in bytes. Requires
// WithRunMemory and must be >= the memory value.
func WithRunMemorySwap(bytes int64) RunOption {
	return func(o *runOptions) { o.MemorySwap, o.memorySwapSet = bytes, true }
}

// WithRunCPUs limits CPUs (--cpus), must be > 0.
func WithRunCPUs(cpus float64) RunOption {
	return func(o *runOptions) { o.CPUs, o.cpusSet = cpus, true }
}

// WithRunPidsLimit sets --pids-limit, must be > 0 (unlimited is refused).
func WithRunPidsLimit(n int64) RunOption {
	return func(o *runOptions) { o.PidsLimit, o.pidsSet = n, true }
}

// WithRunDevice passes one host device (--device). path must be a clean
// absolute path below /dev and must be in the caller-supplied allowlist; the
// device must be usable (/dev/kvm is opened O_RDWR).
func WithRunDevice(devPath string, allow ...string) RunOption {
	return func(o *runOptions) {
		o.Devices = append(o.Devices, runDevice{Path: devPath, Allow: allow})
	}
}

// WithRunUserns sets the user namespace mode. Only "keep-id" is modelled and
// only podman honours it.
func WithRunUserns(mode string) RunOption {
	return func(o *runOptions) { o.Userns = mode }
}

// WithRunPublish publishes a port (-p hostIP:hostPort:ctrPort/proto). hostIP is
// REQUIRED and must be a literal IP; a wildcard address needs
// WithRunPublishAllInterfaces.
func WithRunPublish(hostIP string, hostPort, ctrPort int, proto string) RunOption {
	return func(o *runOptions) {
		o.Publishes = append(o.Publishes, runPublish{hostIP, hostPort, ctrPort, proto})
	}
}

// WithRunPublishAllInterfaces is the explicit opt-in to publish on 0.0.0.0/::.
func WithRunPublishAllInterfaces() RunOption {
	return func(o *runOptions) { o.PublishAllIfaces = true }
}

// WithRunLabel adds a container label (--label k=v).
func WithRunLabel(key, value string) RunOption {
	return func(o *runOptions) {
		if o.Labels == nil {
			o.Labels = make(map[string]string)
		}
		o.Labels[key] = value
	}
}

// WithRunImageDigest requires the image reference to be pinned by a sha256
// digest (name@sha256:<64 hex>); a mutable tag is refused.
func WithRunImageDigest() RunOption {
	return func(o *runOptions) { o.RequireDigest = true }
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrRunInvalidOption, fmt.Sprintf(format, a...))
}

func forbidden(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrRunForbiddenOption, fmt.Sprintf(format, a...))
}

// validateRunOptions checks every typed option plus the free-form forms and
// returns the first problem. It runs before any argv is built.
func validateRunOptions(o *runOptions, image string) error {
	if strings.HasPrefix(image, "-") {
		return forbidden("image reference %q parses as a flag", image)
	}
	if err := validatePolicy(o); err != nil {
		return err
	}
	if o.memorySet && o.Memory <= 0 {
		return invalid("memory must be > 0 bytes, got %d", o.Memory)
	}
	if o.memorySwapSet {
		if !o.memorySet {
			return invalid("memory-swap requires memory")
		}
		if o.MemorySwap < o.Memory {
			return invalid("memory-swap %d is below memory %d", o.MemorySwap, o.Memory)
		}
	}
	if o.cpusSet && !(o.CPUs > 0) {
		return invalid("cpus must be > 0, got %g", o.CPUs)
	}
	if o.pidsSet && o.PidsLimit <= 0 {
		return invalid("pids-limit must be > 0, got %d", o.PidsLimit)
	}
	for _, d := range o.Devices {
		if d.Path != path.Clean(d.Path) || !strings.HasPrefix(d.Path, "/dev/") ||
			strings.ContainsAny(d.Path, ":, \t\n") {
			return invalid("device %q is not a clean absolute path below /dev", d.Path)
		}
		allowed := false
		for _, a := range d.Allow {
			if a == d.Path {
				allowed = true
			}
		}
		if !allowed {
			return invalid("device %q is not in the caller allowlist", d.Path)
		}
		if err := deviceUsable(d.Path); err != nil {
			return fmt.Errorf("device %s is not usable: %w", d.Path, err)
		}
	}
	if o.Userns != "" {
		if o.Userns != "keep-id" {
			return invalid("userns %q is not modelled (only keep-id)", o.Userns)
		}
		if o.runtimeName != "" && o.runtimeName != "podman" {
			return fmt.Errorf("%w: --userns=keep-id on %s", ErrRunUnsupportedOption, o.runtimeName)
		}
	}
	for _, p := range o.Publishes {
		ip := net.ParseIP(p.HostIP)
		if ip == nil {
			return invalid("publish host ip %q is not a literal IP", p.HostIP)
		}
		if ip.IsUnspecified() && !o.PublishAllIfaces {
			return invalid("publish on %s needs WithRunPublishAllInterfaces", p.HostIP)
		}
		if p.Proto != "tcp" && p.Proto != "udp" {
			return invalid("publish proto %q", p.Proto)
		}
		if p.CtrPort < 1 || p.CtrPort > 65535 || p.HostPort < 0 || p.HostPort > 65535 {
			return invalid("publish ports %d:%d out of range", p.HostPort, p.CtrPort)
		}
	}
	for k := range o.Labels {
		if k == "" || strings.Contains(k, "=") {
			return invalid("label key %q", k)
		}
	}
	if o.RequireDigest && !digestRef.MatchString(image) {
		return invalid("image %q is not pinned by @sha256:<64 hex>", image)
	}
	return nil
}

// RunOption configures one-shot container execution.
type RunOption func(*runOptions)

// defaultRunOptions models the shape the two real consumers need: a one-shot
// `run --rm -i` that streams a document in and is gone afterwards. Remove
// defaults to TRUE because the alternative — leaking a stopped container per
// invocation — is a resource leak a caller has to remember to avoid, and the
// primitive exists precisely for throwaway work.
func defaultRunOptions() *runOptions {
	return &runOptions{
		Remove: true,
		Env:    make(map[string]string),
	}
}

func applyRunOptions(opts []RunOption) *runOptions {
	o := defaultRunOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithRunStdin streams r into the container's standard input.
//
// Supplying it makes the runtime pass the CLI's interactive-stdin flag (-i),
// so the container sees a real pipe rather than a closed descriptor. If the
// runtime's executor cannot stream stdin, Run fails with ErrStdinUnsupported
// rather than running the command without it.
func WithRunStdin(r io.Reader) RunOption {
	return func(o *runOptions) {
		o.Stdin = r
	}
}

// WithRunRemove sets whether the container is deleted once it exits (--rm).
// Defaults to true.
func WithRunRemove(remove bool) RunOption {
	return func(o *runOptions) {
		o.Remove = remove
	}
}

// WithRunName assigns a name to the ephemeral container (--name).
func WithRunName(name string) RunOption {
	return func(o *runOptions) {
		o.Name = name
	}
}

// WithRunEntrypoint overrides the image's entrypoint (--entrypoint).
func WithRunEntrypoint(entrypoint string) RunOption {
	return func(o *runOptions) {
		o.Entrypoint = entrypoint
	}
}

// WithRunWorkDir sets the working directory inside the container (-w).
func WithRunWorkDir(dir string) RunOption {
	return func(o *runOptions) {
		o.WorkDir = dir
	}
}

// WithRunUser sets the user the container process runs as (-u).
func WithRunUser(user string) RunOption {
	return func(o *runOptions) {
		o.User = user
	}
}

// WithRunNetwork attaches the container to a named network (--network).
func WithRunNetwork(network string) RunOption {
	return func(o *runOptions) {
		o.Network = network
	}
}

// WithRunEnv adds environment variables for the ephemeral container (-e).
func WithRunEnv(env map[string]string) RunOption {
	return func(o *runOptions) {
		for k, v := range env {
			o.Env[k] = v
		}
	}
}

// WithRunVolumes adds mounts in the CLI's "source:target[:ro|rw]" form. They are
// PARSED into typed mounts and subject to the same source policy as
// WithRunMount (bind sources need WithRunMountAllowedRoots); the argv uses
// --mount, never -v.
func WithRunVolumes(volumes ...string) RunOption {
	return func(o *runOptions) {
		o.Volumes = append(o.Volumes, volumes...)
	}
}

// WithRunExtraArgs appends flags immediately before the image name. ONLY the
// flags in the allowlist (see README "Run option allowlist") are accepted;
// every other token is refused with ErrRunForbiddenOption. Mounts, capabilities
// and security options are NOT available here: use the typed options.
func WithRunExtraArgs(args ...string) RunOption {
	return func(o *runOptions) {
		o.ExtraArgs = append(o.ExtraArgs, args...)
	}
}

// buildRunArgs assembles the argv for a Docker-CLI-compatible `run`, shared by
// the docker, podman and nerdctl runtimes, whose run subcommands take the same
// flags. Env keys are emitted in sorted order so the argv is deterministic —
// Go map iteration is randomised, and a non-deterministic argv makes a test
// that asserts on it flaky rather than wrong.
func buildRunArgs(o *runOptions, image string, cmd []string) []string {
	args := []string{"run"}
	if o.Remove {
		args = append(args, "--rm")
	}
	if o.Stdin != nil {
		args = append(args, "-i")
	}
	if o.Name != "" {
		args = append(args, "--name", o.Name)
	}
	if o.Entrypoint != "" {
		args = append(args, "--entrypoint", o.Entrypoint)
	}
	if o.WorkDir != "" {
		args = append(args, "-w", o.WorkDir)
	}
	if o.User != "" {
		args = append(args, "-u", o.User)
	}
	if o.Network != "" {
		args = append(args, "--network", o.Network)
	}
	if o.memorySet {
		args = append(args, "--memory", strconv.FormatInt(o.Memory, 10))
		swap := o.Memory
		if o.memorySwapSet {
			swap = o.MemorySwap
		}
		args = append(args, "--memory-swap", strconv.FormatInt(swap, 10))
	}
	if o.cpusSet {
		args = append(args, "--cpus", strconv.FormatFloat(o.CPUs, 'g', -1, 64))
	}
	if o.pidsSet {
		args = append(args, "--pids-limit", strconv.FormatInt(o.PidsLimit, 10))
	}
	for _, d := range o.Devices {
		args = append(args, "--device", d.Path)
	}
	if o.Userns != "" {
		args = append(args, "--userns", o.Userns)
	}
	for _, p := range o.Publishes {
		args = append(args, "-p", fmt.Sprintf("%s:%d:%d/%s", p.HostIP, p.HostPort, p.CtrPort, p.Proto))
	}
	lkeys := make([]string, 0, len(o.Labels))
	for k := range o.Labels {
		lkeys = append(lkeys, k)
	}
	sort.Strings(lkeys)
	for _, k := range lkeys {
		args = append(args, "--label", k+"="+o.Labels[k])
	}
	keys := make([]string, 0, len(o.Env))
	for k := range o.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, o.Env[k]))
	}
	args = append(args, o.resolvedMnts...)
	if o.CapDropAll {
		args = append(args, "--cap-drop=ALL")
	}
	for _, c := range o.CapAdd {
		args = append(args, "--cap-add="+c)
	}
	if o.NoNewPrivs {
		args = append(args, "--security-opt=no-new-privileges")
	}
	args = append(args, o.ExtraArgs...)
	args = append(args, image)
	return append(args, cmd...)
}

// RunSpec is the resolved form of a Run call: the Docker-CLI-compatible argv
// (beginning with the "run" subcommand, excluding the binary name) and the
// stdin reader, if any.
//
// It exists so a ContainerRuntime implemented OUTSIDE this package —
// remote.RemoteRuntime is the one in this repository — can assemble exactly the
// same command line from the same options instead of re-deriving the flag
// mapping and drifting from it. Stdin is surfaced as a field rather than hidden
// because an implementation that cannot stream it MUST fail loudly, and it can
// only do that if it can see that stdin was asked for.
type RunSpec struct {
	// Args is the argv after the runtime binary, e.g.
	// ["run", "--rm", "-i", "alpine:3.20", "cat"].
	Args []string
	// Stdin is the reader supplied by WithRunStdin, or nil.
	Stdin io.Reader
	// Err is non-nil when the options were refused (typed: ErrRunForbiddenOption,
	// ErrRunUnsupportedOption, ErrRunInvalidOption). Args is nil then; a caller
	// MUST check Err before executing Args.
	Err error
	// Reasons are the documented justifications supplied for reasoned
	// capabilities (WithRunCapAddReason), for the caller's audit trail.
	Reasons []string
}

// ResolveRunSpec applies the given RunOptions and returns the resolved argv and
// stdin reader. See RunSpec.
func ResolveRunSpec(image string, cmd []string, opts []RunOption) RunSpec {
	o := applyRunOptions(opts)
	o.remote = true // the argv runs on another host: no local resolution
	if err := validateRunOptions(o, image); err != nil {
		return RunSpec{Err: err}
	}
	return RunSpec{Args: buildRunArgs(o, image, cmd), Stdin: o.Stdin, Reasons: o.CapReasons}
}

// runViaCLI performs the one-shot run for every Docker-CLI-compatible runtime.
//
// A non-zero container exit is NOT an error: the exit code is data the caller
// asked for, exactly as ExecResult already models it for Exec. An error means
// the run could not be performed — no image name, no stdin support, or the
// binary could not be executed at all.
func runViaCLI(
	ctx context.Context,
	executor CommandExecutor,
	binary, name, image string,
	cmd []string,
	opts []RunOption,
) (*ExecResult, error) {
	if image == "" {
		return nil, fmt.Errorf("%s run: image is required", name)
	}
	o := applyRunOptions(opts)
	o.runtimeName = name
	if err := validateRunOptions(o, image); err != nil {
		return nil, fmt.Errorf("%s run %s: %w", name, image, err)
	}
	args := buildRunArgs(o, image, cmd)

	var (
		stdout, stderr []byte
		exitCode       int
		err            error
	)
	if o.Stdin != nil {
		stdinExec, ok := executor.(StdinExecutor)
		if !ok {
			return nil, fmt.Errorf("%s run %s: %w", name, image, ErrStdinUnsupported)
		}
		stdout, stderr, exitCode, err = stdinExec.ExecuteWithStdin(
			ctx, o.Stdin, binary, args...,
		)
	} else {
		stdout, stderr, exitCode, err = executor.ExecuteWithStderr(
			ctx, binary, args...,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("%s run %s: %w", name, image, err)
	}
	return &ExecResult{
		ExitCode: exitCode,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	}, nil
}
