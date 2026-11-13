package runtime

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Round 3 (STRUCTURAL): the run builder is an ALLOWLIST, not a scan.
//
//   - Free-form ExtraArgs may carry only the flags in extraFlagAllowlist, parsed
//     with the engine's own grammar. Everything else (unknown flags, abbreviated
//     flags, `--` tricks, stray positionals that would become the image) is
//     refused with ErrRunForbiddenOption.
//   - Security-sensitive features (mounts, capabilities, security options,
//     devices, namespaces, publishing) are offered only as typed options. The
//     policy is a gate on the argv this package builds, not a sandbox: it does
//     not inspect what an image, a named volume or an engine default does.
//   - Features that are never available: --rootfs, --env-host, --env-file,
//     --hooks-dir, --volumes-from, --cgroup-parent, --gpus, --privileged,
//     --security-opt other than no-new-privileges, --cap-add outside the typed
//     lists, --mount/-v in ExtraArgs, --device in ExtraArgs, --network in
//     ExtraArgs.

// MountType is the kind of a typed mount. Only bind, volume and tmpfs exist.
type MountType string

// Supported mount types.
const (
	MountBind   MountType = "bind"
	MountVolume MountType = "volume"
	MountTmpfs  MountType = "tmpfs"
)

// RunMount is one typed mount. Source is a host path (bind), a named volume
// (volume; empty = anonymous) or empty (tmpfs).
type RunMount struct {
	Type     MountType
	Source   string
	Target   string
	ReadOnly bool
}

// WithRunMount adds a typed mount. Bind sources must lie below one of the roots
// given with WithRunMountAllowedRoots and outside the built-in denies.
func WithRunMount(m RunMount) RunOption {
	return func(o *runOptions) { o.Mounts = append(o.Mounts, m) }
}

// WithRunMountAllowedRoots sets the path roots below which bind sources may
// live (accumulates). With no roots, every bind mount is refused.
func WithRunMountAllowedRoots(roots ...string) RunOption {
	return func(o *runOptions) { o.MountRoots = append(o.MountRoots, roots...) }
}

// WithRunCapAdd adds capabilities from the safe list (safeCaps).
func WithRunCapAdd(caps ...string) RunOption {
	return func(o *runOptions) { o.CapAdd = append(o.CapAdd, caps...) }
}

// WithRunCapAddReason adds capabilities from the safe list plus the
// reason-gated ones (NET_ADMIN). A documented reason (>= 8 non-blank
// characters) is required and is returned in RunSpec.Reasons.
func WithRunCapAddReason(reason string, caps ...string) RunOption {
	return func(o *runOptions) {
		o.CapAdd = append(o.CapAdd, caps...)
		o.CapReasons = append(o.CapReasons, strings.Join(caps, ",")+": "+reason)
		if o.reasonedCaps == nil {
			o.reasonedCaps = map[string]bool{}
		}
		for _, c := range caps {
			o.reasonedCaps[strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(c)), "CAP_")] = true
		}
	}
}

// WithRunDetach runs the container detached (-d); the engine prints the
// container id on stdout.
func WithRunDetach() RunOption { return func(o *runOptions) { o.Detach = true } }

const (
	minMemoryBytes = 4 << 20
	maxPidsLimit   = 1 << 20
	maxCPUs        = 4096
)

var reMemory = regexp.MustCompile(`^([0-9]{1,15})([bkmg]?)$`)

// ParseRunMemory converts the engine's size spelling ("128m", "1g", "4096",
// lower-case unit b/k/m/g, binary multiples) into bytes for WithRunMemory.
func ParseRunMemory(s string) (int64, error) {
	m := reMemory.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return 0, invalid("memory %q is not <digits>[b|k|m|g]", s)
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	mult := map[string]int64{"": 1, "b": 1, "k": 1 << 10, "m": 1 << 20, "g": 1 << 30}[m[2]]
	if n > (1<<62)/mult {
		return 0, invalid("memory %q overflows", s)
	}
	return n * mult, nil
}

// WithRunCapDropAll drops every capability (--cap-drop=ALL) before any add.
func WithRunCapDropAll() RunOption { return func(o *runOptions) { o.CapDropAll = true } }

// WithRunNoNewPrivileges sets --security-opt=no-new-privileges, the only
// security option the builder offers.
func WithRunNoNewPrivileges() RunOption { return func(o *runOptions) { o.NoNewPrivs = true } }

// safeCaps may be added without justification.
var safeCaps = map[string]bool{
	"NET_BIND_SERVICE": true, "NET_RAW": true, "CHOWN": true, "FOWNER": true,
	"FSETID": true, "KILL": true, "SETGID": true, "SETUID": true, "SYS_CHROOT": true,
}

// reasonCaps need WithRunCapAddReason.
var reasonCaps = map[string]bool{"NET_ADMIN": true}

// ---- free-form flag allowlist ---------------------------------------------

type flagSpec struct {
	short    string // single character or ""
	value    bool   // takes a value
	validate func(string) error
}

var extraFlagAllowlist = map[string]flagSpec{
	"--read-only":    {},
	"--init":         {},
	"--rm":           {},
	"--interactive":  {short: "i"},
	"--label":        {short: "l", value: true, validate: vLabel},
	"--env":          {short: "e", value: true, validate: vEnv},
	"--workdir":      {short: "w", value: true, validate: vAbsPath},
	"--hostname":     {value: true, validate: vHostname},
	"--stop-timeout": {value: true, validate: vIntRange(0, 3600)},
	"--tmpfs":        {value: true, validate: vTmpfs},
	"--dns":          {value: true, validate: vIP},
	"--add-host":     {value: true, validate: vAddHost},
	"--ulimit":       {value: true, validate: vUlimit},
	"--restart":      {value: true, validate: vOneOf("no")},
	"--stop-signal":  {value: true, validate: vSignal},
	"--pull":         {value: true, validate: vOneOf("never", "missing")},
	"--shm-size":     {value: true, validate: vShm},
}

var shortAllowlist = func() map[byte]string {
	m := map[byte]string{}
	for n, s := range extraFlagAllowlist {
		if s.short != "" {
			m[s.short[0]] = n
		}
	}
	return m
}()

func bad(format string, a ...any) error { return invalid(format, a...) }

var (
	reEnvName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reHostname = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	reName     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	reSignal   = regexp.MustCompile(`^(SIG)?[A-Z0-9]{1,12}$`)
	reShm      = regexp.MustCompile(`^[0-9]{1,12}[bkmg]?$`)
	reTmpOpt   = regexp.MustCompile(`^(ro|rw|noexec|exec|nosuid|nodev|size=[0-9]+[bkmg]?|mode=[0-7]{3,4}|uid=[0-9]+|gid=[0-9]+|nr_inodes=[0-9]+[km]?)$`)
	ulimitOK   = map[string]bool{"nofile": true, "nproc": true, "core": true, "cpu": true, "fsize": true, "stack": true}
)

func vLabel(v string) error {
	k, _, ok := strings.Cut(v, "=")
	if !ok || k == "" || strings.ContainsAny(k, " \t\n\x00") {
		return bad("label %q must be key=value", v)
	}
	return nil
}

func vEnv(v string) error {
	k, _, ok := strings.Cut(v, "=")
	if !ok || !reEnvName.MatchString(k) { // NAME alone would copy a host variable
		return bad("env %q must be NAME=value", v)
	}
	return nil
}

func vAbsPath(v string) error {
	if !path.IsAbs(v) || path.Clean(v) != v || strings.ContainsAny(v, "\x00\n") {
		return bad("path %q must be absolute and clean", v)
	}
	return nil
}

func vHostname(v string) error {
	if !reHostname.MatchString(v) {
		return bad("hostname %q", v)
	}
	return nil
}

func vIntRange(lo, hi int) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < lo || n > hi {
			return bad("value %q not an integer in [%d,%d]", v, lo, hi)
		}
		return nil
	}
}

func vOneOf(allowed ...string) func(string) error {
	return func(v string) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return bad("value %q not one of %v", v, allowed)
	}
}

func vSignal(v string) error {
	if n, err := strconv.Atoi(v); err == nil {
		if n >= 1 && n <= 64 {
			return nil
		}
	} else if reSignal.MatchString(v) {
		return nil
	}
	return bad("signal %q", v)
}

func vShm(v string) error {
	if !reShm.MatchString(v) {
		return bad("shm-size %q", v)
	}
	return nil
}

func vIP(v string) error {
	if net.ParseIP(v) == nil {
		return bad("%q is not a literal IP", v)
	}
	return nil
}

func vAddHost(v string) error {
	h, ip, ok := strings.Cut(v, ":")
	if !ok || !reHostname.MatchString(h) || (ip != "host-gateway" && net.ParseIP(ip) == nil) {
		return bad("add-host %q must be host:ip or host:host-gateway", v)
	}
	return nil
}

func vUlimit(v string) error {
	name, lim, ok := strings.Cut(v, "=")
	if !ok || !ulimitOK[name] {
		return bad("ulimit %q: name not allowed", v)
	}
	for _, p := range strings.Split(lim, ":") {
		if n, err := strconv.ParseUint(p, 10, 32); err != nil || n > 1<<30 {
			return bad("ulimit %q: bad limit", v)
		}
	}
	if strings.Count(lim, ":") > 1 {
		return bad("ulimit %q", v)
	}
	return nil
}

func vTmpfs(v string) error {
	dst, opts, _ := strings.Cut(v, ":")
	if err := containerTarget(dst); err != nil {
		return err
	}
	if opts != "" {
		for _, p := range strings.Split(opts, ",") {
			if !reTmpOpt.MatchString(p) {
				return bad("tmpfs option %q not allowed", p)
			}
		}
	}
	return nil
}

// containerTarget validates a path INSIDE the container.
func containerTarget(t string) error {
	if err := vAbsPath(t); err != nil {
		return err
	}
	if t == "/" || t == "/proc" || strings.HasPrefix(t, "/proc/") || t == "/sys" ||
		strings.HasPrefix(t, "/sys/") || strings.ContainsAny(t, ",\"\r\n\t") {
		return bad("container path %q not allowed", t)
	}
	return nil
}

// scanExtraArgs parses ExtraArgs with pflag-like grammar against the allowlist.
func scanExtraArgs(args []string) error {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--"): // "--" alone is not a listed flag either
			name, val, hasVal := strings.Cut(a, "=")
			spec, ok := extraFlagAllowlist[name]
			if !ok {
				return forbidden("extra arg %q is not on the flag allowlist", a)
			}
			if !spec.value {
				if hasVal {
					return forbidden("extra arg %q: boolean flags take no value", a)
				}
				continue
			}
			if !hasVal {
				if i+1 >= len(args) {
					return invalid("extra arg %s needs a value", name)
				}
				i++
				val = args[i]
			}
			if err := spec.validate(val); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for j := 1; j < len(a); j++ {
				long, ok := shortAllowlist[a[j]]
				if !ok {
					return forbidden("extra arg %q: short flag -%c is not on the allowlist", a, a[j])
				}
				spec := extraFlagAllowlist[long]
				if !spec.value {
					continue
				}
				val := strings.TrimPrefix(a[j+1:], "=")
				if a[j+1:] == "" {
					if i+1 >= len(args) {
						return invalid("extra arg -%c needs a value", a[j])
					}
					i++
					val = args[i]
				}
				if err := spec.validate(val); err != nil {
					return fmt.Errorf("%s: %w", long, err)
				}
				break
			}
		default:
			// A stray positional would be taken as the image by the engine.
			return forbidden("extra arg %q is a positional, not a flag", a)
		}
	}
	return nil
}

// ---- network ---------------------------------------------------------------

func validateNetwork(n string) error {
	l := strings.ToLower(strings.TrimSpace(n))
	switch {
	case l == "host" || l == "container" || l == "ns" || l == "private-host":
		return forbidden("network %q shares a host/foreign namespace", n)
	case !reName.MatchString(n): // also refuses ":" / "=" option and ns: forms
		return forbidden("network %q is not a plain network name", n)
	}
	return nil
}

// ---- mounts ----------------------------------------------------------------

var homeDir = func() string { h, _ := os.UserHomeDir(); return h }

var denySubtrees = []string{
	"/proc", "/sys", "/dev", "/run", "/var", "/etc", "/boot", "/root", "/usr",
	"/opt", "/srv", "/mnt", "/media", "/snap", "/lib", "/lib32", "/lib64", "/libx32", "/bin", "/sbin",
}

var denyTmpPrefixes = []string{"storage-run-", "podman-run-", "containers-user-", "podman-",
	"tmux-", "ssh-", ".X11-unix", "systemd-private-"}

var denyHomeChildren = []string{
	".config/gh", ".git-credentials", ".local/share/keyrings", ".pgpass",
	".ssh", ".gnupg", ".aws", ".kube", ".docker", ".config/containers",
	".bashrc", ".bash_profile", ".profile", ".zshrc", ".netrc", ".config/systemd",
	".config/autostart", ".local/bin",
	".local/share/containers", ".local/share/docker", ".password-store",
}

// ---- host-path deny: ONE class-wide rule (CONT-DENY-STRUCT) ----------------
//
// Protected anchors: /home (the filesystem root is an ancestor of every anchor), the tmp anchor (all "node"
// anchors), the user home (node), every credential child of the home (tree)
// and every system subtree (tree). Each anchor is expanded to its forms
// {lexical, fully symlink-resolved}; the resolution is
// tolerant of non-existent tails (longest existing prefix is resolved).
// A candidate P (likewise expanded) is DENIED iff, for ANY pair of forms,
//
//	tree anchor A: P == A, or P is under A, or A is under P (ancestor mount)
//	node anchor A: P == A, or A is under P (ancestor mount) - but P below
//	               a node anchor (a work dir under the home) is ACCEPTED.
//
// So the only ancestors of the home that may be mounted: none. A work dir
// under the home that is neither an ancestor nor a descendant of a protected
// anchor is accepted. Local mode with no resolvable home is refused by the
// caller (bindSource). Remote mode (local=false) is purely lexical.

// tmpAnchor is the shared temp directory whose runtime-state children are
// protected. It is a variable only as a test seam.
var tmpAnchor = "/tmp"

// fallbackHome is the passwd-derived home used when $HOME is unusable.
var fallbackHome = func() string {
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return ""
}

// localHomes returns every usable home of the current user for a local
// runtime: $HOME and the passwd home, each only if absolute, clean and not "/".
// An unusable one is skipped (the other still protects); if neither is usable
// the result is an error (fail closed: an unknown home would drop every
// home/credential deny).
func localHomes() ([]string, error) {
	var out []string
	for _, h := range []string{homeDir(), fallbackHome()} {
		if h == "" || h == "/" || !filepath.IsAbs(h) || filepath.Clean(h) != h {
			continue
		}
		dup := false
		for _, e := range out {
			if e == h {
				dup = true
			}
		}
		if !dup {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		return nil, forbidden("cannot determine a valid home directory; refusing bind mounts (fail closed)")
	}
	return out, nil
}

const maxSymlinkHops = 40

// resolveWalk resolves p component by component with Lstat/Readlink. A missing
// component ends resolution (the rest is kept lexically). A symlink - existing
// target or DANGLING - is always followed (relative targets are taken relative
// to the directory holding the link). More than maxSymlinkHops links (a loop)
// yields ok=false: the caller fails closed.
//
// locs lists every PHYSICAL location the walk visited: for each component
// (also inside a link target, at every hop) the resolved parent joined with the
// component name. A container that can write to a directory holding such an
// entry can replace it (a symlink swapped for another one), so locs are node
// anchors for the deny rule.
func resolveWalk(p string) (res string, locs []string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(p), "/"), "/")
	cur := "/"
	hops := 0
	for len(parts) > 0 {
		c := parts[0]
		parts = parts[1:]
		if c == "" || c == "." {
			continue
		}
		if c == ".." {
			cur = filepath.Dir(cur)
			continue
		}
		nx := filepath.Join(cur, c)
		locs = append(locs, nx)
		fi, err := os.Lstat(nx)
		if err != nil {
			cur = nx
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			hops++
			if hops > maxSymlinkHops {
				return "", locs, false
			}
			t, err := os.Readlink(nx)
			if err != nil {
				return "", locs, false
			}
			if filepath.IsAbs(t) {
				cur = "/"
			}
			parts = append(strings.Split(strings.TrimPrefix(t, "/"), "/"), parts...)
			continue
		}
		cur = nx
	}
	return cur, locs, true
}

// pathFormsOK returns the lexical and the fully-resolved form plus the
// physical locations visited. ok=false means resolution failed (symlink loop
// or too many hops): fail closed.
func pathFormsOK(p string, local bool) (forms, locs []string, ok bool) {
	forms = []string{p}
	if !local {
		return forms, nil, true
	}
	r, locs, ok := resolveWalk(p)
	if !ok {
		return forms, locs, false
	}
	if r != p {
		forms = append(forms, r)
	}
	return forms, locs, true
}

func pathForms(p string, local bool) []string {
	f, _, _ := pathFormsOK(p, local)
	return f
}

func under(p, root string) bool {
	if root == "/" {
		return p != "/"
	}
	return strings.HasPrefix(p, root+"/")
}

type protAnchor struct {
	path string
	tree bool // tree: P under A is also denied
	why  string
}

func protectedAnchors(homes []string) []protAnchor {
	as := []protAnchor{
		{"/home", false, "system root directory"},
		{tmpAnchor, false, "system root directory"},
	}
	for _, home := range homes {
		as = append(as, protAnchor{home, false, "the user home directory or an ancestor of it"})
		for _, c := range denyHomeChildren {
			as = append(as, protAnchor{filepath.Join(home, c), true, "credential/container-storage directory in home (or an ancestor of it)"})
		}
	}
	for _, sub := range denySubtrees {
		as = append(as, protAnchor{sub, true, "system directory"})
	}
	return as
}

// denyHostPath reports whether a CLEAN ABSOLUTE host path is never mountable.
// home == "" selects the purely lexical remote-side check.
func denyHostPath(p, home string) (bool, string) {
	return denyHostPathMode(p, home, home != "")
}

func denyHostPathMode(p, home string, local bool) (bool, string) {
	var homes []string
	if home != "" {
		homes = []string{home}
	}
	return denyHostPathHomes(p, homes, local)
}

func denyHostPathHomes(p string, homes []string, local bool) (bool, string) {
	pf, _, pok := pathFormsOK(p, local)
	if !pok {
		return true, "path does not resolve (symlink loop); refused (fail closed)"
	}
	for _, x := range pf {
		if strings.HasPrefix(x, "/home/") && strings.Count(x[1:], "/") == 1 {
			return true, "a user home directory"
		}
		if strings.HasSuffix(x, ".sock") || strings.HasSuffix(x, ".socket") {
			return true, "socket file"
		}
	}
	for _, a := range protectedAnchors(homes) {
		afs, alocs, aok := pathFormsOK(a.path, local)
		if !aok {
			// an anchor that cannot be resolved (loop / too many hops) cannot be
			// protected precisely: refuse everything (fail closed)
			return true, "protected path " + a.path + " does not resolve (symlink loop); refused (fail closed)"
		}
		for _, af := range afs {
			for _, x := range pf {
				if x == af || under(af, x) || (a.tree && under(x, af)) {
					return true, a.why + " (" + af + ")"
				}
			}
		}
		// physical locations the anchor's resolution passed through: a candidate
		// equal to or above one could replace the link that leads to the anchor
		for _, l := range alocs {
			for _, x := range pf {
				if x == l || under(l, x) {
					return true, a.why + " (location " + l + " on the way to " + a.path + ")"
				}
			}
		}
	}
	for _, tf := range pathForms(tmpAnchor, local) {
		for _, x := range pf {
			if under(x, tf) {
				first := strings.SplitN(x[len(tf)+1:], "/", 2)[0]
				for _, pre := range denyTmpPrefixes {
					if strings.HasPrefix(first, pre) {
						return true, "container runtime or session state in " + tf
					}
				}
			}
		}
	}
	return false, ""
}

func within(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }

// bindSource validates (and for a local runtime resolves) one bind source and
// returns the path to put in the argv.
func bindSource(o *runOptions, src string) (string, error) {
	if !filepath.IsAbs(src) || filepath.Clean(src) != src || strings.ContainsAny(src, ",\"\n\r\x00") {
		return "", forbidden("bind source %q must be an absolute, clean, literal path", src)
	}
	var homes []string
	if !o.remote {
		hs, err := localHomes()
		if err != nil {
			return "", err
		}
		homes = hs
	}
	if len(o.MountRoots) == 0 {
		return "", forbidden("bind mount %q: no WithRunMountAllowedRoots given", src)
	}
	roots := make([]string, 0, len(o.MountRoots))
	for _, r := range o.MountRoots {
		if !filepath.IsAbs(r) || filepath.Clean(r) != r {
			return "", invalid("allowed root %q must be absolute and clean", r)
		}
		if d, why := denyHostPathHomes(r, homes, !o.remote); d {
			return "", forbidden("allowed root %q is itself denied (%s); mount a dedicated subdirectory instead", r, why)
		}
		if !o.remote {
			rr, err := filepath.EvalSymlinks(r)
			if err != nil {
				return "", invalid("allowed root %q does not resolve: %v", r, err)
			}
			r = rr // denyHostPathMode above already checked the resolved forms
		}
		roots = append(roots, r)
	}
	cands := []string{src}
	use := src
	if !o.remote {
		r, err := filepath.EvalSymlinks(src)
		if err != nil {
			return "", forbidden("bind source %q does not resolve: %v", src, err)
		}
		if fi, err := os.Stat(r); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return "", forbidden("bind source %q is a socket", src)
		}
		use = r // pass the RESOLVED path (a later symlink swap cannot redirect it); its forms are denied below
	}
	for _, c := range cands {
		if d, why := denyHostPathHomes(c, homes, !o.remote); d {
			return "", forbidden("bind source %q refused (%s); mount a dedicated subdirectory instead", c, why)
		}
	}
	ok := false
	for _, r := range roots {
		if within(use, r) {
			ok = true
		}
	}
	if !ok {
		return "", forbidden("bind source %q is outside the allowed roots", use)
	}
	return use, nil
}

func parseVolumeSpec(spec string) (RunMount, error) {
	parts := strings.Split(spec, ":")
	if len(parts) > 3 || spec == "" {
		return RunMount{}, invalid("volume %q must be source:target[:ro|rw]", spec)
	}
	m := RunMount{}
	switch len(parts) {
	case 1:
		m.Type, m.Target = MountVolume, parts[0]
	default:
		m.Source, m.Target = parts[0], parts[1]
		if len(parts) == 3 {
			switch parts[2] {
			case "ro":
				m.ReadOnly = true
			case "rw":
			default:
				return RunMount{}, invalid("volume option %q not allowed (only ro, rw)", parts[2])
			}
		}
		if strings.HasPrefix(m.Source, "/") || strings.HasPrefix(m.Source, ".") || strings.HasPrefix(m.Source, "~") {
			m.Type = MountBind
		} else {
			m.Type = MountVolume
		}
	}
	return m, nil
}

func (o *runOptions) buildMounts() error {
	all := append([]RunMount(nil), o.Mounts...)
	for _, v := range o.Volumes {
		m, err := parseVolumeSpec(v)
		if err != nil {
			return err
		}
		all = append(all, m)
	}
	o.resolvedMnts = nil
	for _, m := range all {
		if err := containerTarget(m.Target); err != nil {
			return fmt.Errorf("mount target: %w", err)
		}
		var kv []string
		switch m.Type {
		case MountBind:
			src, err := bindSource(o, m.Source)
			if err != nil {
				return err
			}
			kv = []string{"type=bind", "source=" + src, "target=" + m.Target}
		case MountVolume:
			if m.Source != "" && !reName.MatchString(m.Source) {
				return forbidden("named volume %q is not a plain name", m.Source)
			}
			kv = []string{"type=volume"}
			if m.Source != "" {
				kv = append(kv, "source="+m.Source)
			}
			kv = append(kv, "target="+m.Target)
		case MountTmpfs:
			if m.Source != "" {
				return invalid("tmpfs mount takes no source")
			}
			kv = []string{"type=tmpfs", "target=" + m.Target}
		default:
			return forbidden("mount type %q is not available (bind, volume, tmpfs only)", m.Type)
		}
		if m.ReadOnly {
			kv = append(kv, "readonly")
		}
		o.resolvedMnts = append(o.resolvedMnts, "--mount", strings.Join(kv, ","))
	}
	return nil
}

// ---- capabilities ----------------------------------------------------------

func (o *runOptions) normaliseCaps() error {
	seen := map[string]bool{}
	for _, r := range o.CapReasons {
		_, why, _ := strings.Cut(r, ": ")
		if len(strings.TrimSpace(why)) < 8 {
			return invalid("capability reason %q is too short (>= 8 characters)", r)
		}
	}
	for _, c := range o.CapAdd {
		n := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(c)), "CAP_")
		switch {
		case safeCaps[n]:
		case reasonCaps[n]:
			if !o.reasonedCaps[n] {
				return forbidden("capability %s needs WithRunCapAddReason", n)
			}
		default:
			return forbidden("capability %q is not on the capability allowlist", c)
		}
		seen[n] = true
	}
	o.CapAdd = o.CapAdd[:0]
	for n := range seen {
		o.CapAdd = append(o.CapAdd, n)
	}
	sort.Strings(o.CapAdd)
	return nil
}

// validatePolicy runs every allowlist rule of the round-3 design.
func validatePolicy(o *runOptions) error {
	if o.Network != "" {
		if err := validateNetwork(o.Network); err != nil {
			return err
		}
	}
	if err := scanExtraArgs(o.ExtraArgs); err != nil {
		return err
	}
	if o.Name != "" && !reName.MatchString(o.Name) {
		return invalid("container name %q", o.Name)
	}
	if o.WorkDir != "" {
		if err := vAbsPath(o.WorkDir); err != nil {
			return err
		}
	}
	for k := range o.Env {
		if !reEnvName.MatchString(k) {
			return invalid("env name %q", k)
		}
	}
	if err := o.buildMounts(); err != nil {
		return err
	}
	return o.normaliseCaps()
}
