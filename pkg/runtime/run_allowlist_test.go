//go:build !integration

package runtime

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Round 3 (STRUCTURAL): allowlist design. Every member of the round-2 review
// classes R2-1..R2-4/R2-7/R2-8 has a RED row here. PROVISIONAL.

// localMode makes the helpers resolve like the local CLI runtimes (symlinks
// resolved, files stat'd) instead of ResolveRunSpec's remote semantics.
var localMode bool

func useLocal(t *testing.T) {
	localMode = true
	t.Cleanup(func() { localMode = false })
}

func resolveSpec(opts []RunOption) RunSpec {
	if !localMode {
		return ResolveRunSpec("alpine:3.20", nil, opts)
	}
	o := applyRunOptions(opts)
	o.runtimeName = "podman"
	if err := validateRunOptions(o, "alpine:3.20"); err != nil {
		return RunSpec{Err: err}
	}
	return RunSpec{Args: buildRunArgs(o, "alpine:3.20", nil)}
}

func refused(t *testing.T, name string, opts ...RunOption) {
	t.Helper()
	spec := resolveSpec(opts)
	require.Error(t, spec.Err, name)
	assert.True(t, errors.Is(spec.Err, ErrRunForbiddenOption), "%s: %v", name, spec.Err)
	assert.Nil(t, spec.Args, name)
}

func invalidOpt(t *testing.T, name string, opts ...RunOption) {
	t.Helper()
	spec := resolveSpec(opts)
	require.Error(t, spec.Err, name)
	assert.True(t, errors.Is(spec.Err, ErrRunInvalidOption), "%s: %v", name, spec.Err)
	assert.Nil(t, spec.Args, name)
}

func allowed(t *testing.T, name string, opts ...RunOption) []string {
	t.Helper()
	spec := resolveSpec(opts)
	assert.NoError(t, spec.Err, name)
	return spec.Args
}

func extra(a ...string) RunOption { return WithRunExtraArgs(a...) }

func TestAL_ExtraArgs_UnlistedFlagsRefused_AllSpellings(t *testing.T) {
	cases := [][]string{
		// R2-1 mounts / R2-8 rootfs and friends
		{"--mount", "type=glob,src=/etc/*,target=/x"},
		{"--mount=type=glob,src=/etc/*,target=/x"},
		{"--mount", "type=volume,volume-opt=device=/,target=/x"},
		{"--mount", "type=bind,source=/tmp/x,target=/x"},
		{"--mount", "src=/etc,target=/x"}, // no type=
		{"-v", "/etc:/x"}, {"-v=/etc:/x"}, {"-v/etc:/x"}, {"--volume", "/etc:/x"}, {"--volume=/etc:/x"},
		{"--rootfs", "/"}, {"--rootfs=/"},
		{"--volumes-from", "other"}, {"--volumes-from=other"},
		{"--cgroup-parent", "/"}, {"--cgroup-parent=system.slice"},
		{"--env-host"}, {"--env-host=true"}, {"--env-file", "/etc/environment"},
		{"--hooks-dir", "/x"}, {"--hooks-dir=/x"},
		{"--gpus", "all"}, {"--gpus=all"},
		// R2-3 security opts
		{"--security-opt", "label=type:spc_t"}, {"--security-opt=label=type:spc_t"},
		{"--security-opt=unmask=ALL"}, {"--security-opt", "seccomp=/path"},
		{"--security-opt=label:disable"}, {"--security-opt=no-new-privileges"},
		{"--security-opt=seccomp=unconfined"},
		// R2-2/R2-4 capabilities in free-form args
		{"--cap-add=DAC_READ_SEARCH"}, {"--cap-add", "BPF"}, {"--cap-add=NET_ADMIN"},
		{"--cap-add=SYS_ADMIN"}, {"--cap-drop=ALL"},
		// namespaces / devices / privileges
		{"--privileged"}, {"--privileged=true"}, {"--network=host"}, {"--network", "host"},
		{"--net=host"}, {"--network=pasta:--map-gw"}, {"--network=pasta"},
		{"--userns=host"}, {"--userns=keep-id"}, {"--pid=host"}, {"--ipc=host"}, {"--uts=host"},
		{"--cgroupns=host"}, {"--device", "/dev/mem"}, {"--device=/dev/kvm"},
		{"--device-cgroup-rule=a *:* rwm"},
		{"--publish", "0.0.0.0:80:80"}, {"-p", "80:80"}, {"--memory", "1g"},
		{"--entrypoint", "sh"}, {"--user", "0"}, {"-u", "0"}, {"--name", "x"},
		// grammar tricks
		{"--"}, {"--", "--read-only"}, {"-"}, {"stray"}, {"--read-only", "stray"},
		{"--READ-ONLY"}, {"--read"}, {"--read-only=true"}, {"--read-only=false"}, {"--init=false"},
		{"-x"}, {"-vx"}, {"-ix"}, {"-it"}, {"-iv", "/etc:/x"}, {"-ie", "A=b", "-v", "/etc:/x"},
		{"--workdir"}, {"--env"}, {"-e"}, {"--label=--privileged"},
	}
	for _, c := range cases {
		spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{extra(c...)})
		require.Error(t, spec.Err, "%q", c)
		assert.True(t, errors.Is(spec.Err, ErrRunForbiddenOption) || errors.Is(spec.Err, ErrRunInvalidOption),
			"%q: %v", c, spec.Err)
		assert.Nil(t, spec.Args, "%q", c)
	}
}

func TestAL_ExtraArgs_AllowedFlagsAndGrammar(t *testing.T) {
	good := [][]string{
		{"--read-only"}, {"--init"}, {"--rm"}, {"--interactive"}, {"-i"},
		{"--label", "a=b"}, {"--label=a=b"}, {"-l", "a=b"}, {"-la=b"}, {"-l=a=b"},
		{"--env", "A=b"}, {"-e", "A=b"}, {"-eA=b"}, {"-ie", "A=b"}, {"-iel=x"},
		{"--workdir=/w"}, {"-w", "/w"}, {"--hostname", "h-1.example"},
		{"--stop-timeout=5"}, {"--tmpfs", "/scratch:rw,noexec,size=64m"}, {"--tmpfs=/dev/shm"},
		{"--dns", "1.1.1.1"}, {"--dns=::1"}, {"--add-host", "a.b:10.0.0.1"}, {"--add-host=h:host-gateway"},
		{"--ulimit", "nofile=1024:2048"}, {"--restart=no"}, {"--stop-signal", "SIGTERM"},
		{"--pull=never"}, {"--pull", "missing"}, {"--shm-size=64m"},
		{"--read-only", "--init", "--label", "x=y", "--label", "z=w"}, // repeated
	}
	for _, g := range good {
		spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{extra(g...)})
		assert.NoError(t, spec.Err, "%q", g)
	}
	// bad VALUES of allowed flags are invalid, not silently passed
	for _, g := range [][]string{
		{"--env", "HOSTVAR"}, {"--env=1A=b"}, {"--label", "noequals"}, {"--label", "=v"}, {"--workdir", "rel"},
		{"--workdir=/a/../b"}, {"--stop-timeout=99999"}, {"--tmpfs", "/proc"}, {"--tmpfs=/x:suid"},
		{"--tmpfs=/x:dev"}, {"--dns", "name"}, {"--add-host", "h:name"}, {"--ulimit", "memlock=-1"},
		{"--ulimit", "nproc=-1"}, {"--restart=always"}, {"--pull=always"}, {"--hostname", "-bad"},
		{"--shm-size", "1;rm"},
	} {
		spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{extra(g...)})
		require.Error(t, spec.Err, "%q", g)
		assert.True(t, errors.Is(spec.Err, ErrRunInvalidOption), "%q: %v", g, spec.Err)
	}
}

func TestAL_TypedCaps(t *testing.T) {
	for _, c := range []string{"DAC_READ_SEARCH", "BPF", "PERFMON", "SYS_BOOT", "SYS_TIME", "MKNOD",
		"NET_ADMIN", "SYS_ADMIN", "ALL", "SYS_MODULE", "SYS_RAWIO", "SYS_PTRACE", "CAP_SYS_ADMIN",
		"DAC_OVERRIDE", "SYS_NICE", "LINUX_IMMUTABLE", "all", "net_admin", "NET_BIND_SERVICE,SYS_ADMIN"} {
		refused(t, "cap "+c, WithRunCapAdd(c))
		if c != "NET_ADMIN" && c != "net_admin" {
			refused(t, "cap+reason "+c, WithRunCapAddReason("documented reason here", c))
		}
	}
	// NET_ADMIN only with a documented reason
	refused(t, "net_admin no reason", WithRunCapAdd("NET_ADMIN"))
	invalidOpt(t, "short reason", WithRunCapAddReason("x", "NET_ADMIN"))
	invalidOpt(t, "blank reason", WithRunCapAddReason("          ", "NET_ADMIN"))
	args := allowed(t, "net_admin reason", WithRunCapAddReason("test needs a tun device", "net_admin", "CAP_NET_BIND_SERVICE"))
	assert.Contains(t, args, "--cap-add=NET_ADMIN")
	assert.Contains(t, args, "--cap-add=NET_BIND_SERVICE")
	args = allowed(t, "drop all + safe", WithRunCapDropAll(), WithRunCapAdd("CHOWN", "chown"), WithRunNoNewPrivileges())
	assert.Contains(t, args, "--cap-drop=ALL")
	assert.Contains(t, args, "--security-opt=no-new-privileges")
	n := 0
	for _, a := range args {
		if a == "--cap-add=CHOWN" {
			n++
		}
	}
	assert.Equal(t, 1, n, "deduplicated")
}

func TestAL_TypedNetwork(t *testing.T) {
	for _, n := range []string{"host", "HOST", " host", "container", "container:x", "ns:/proc/1/ns/net",
		"pasta:--map-gw", "pasta:-T,1", "slirp4netns:allow_host_loopback=true", "a=b", "x,y", "-flag", "bridge:ip=1.2.3.4", "../x"} {
		refused(t, "network "+n, WithRunNetwork(n))
	}
	for _, n := range []string{"none", "bridge", "pasta", "slirp4netns", "mynet", "hostnet", "my-net_1.x"} {
		allowed(t, "network "+n, WithRunNetwork(n))
	}
}

// ---- mounts ----------------------------------------------------------------

func fakeHome(t *testing.T, h string) {
	old := homeDir
	homeDir = func() string { return h }
	t.Cleanup(func() { homeDir = old })
}

func TestAL_Mounts_TypedPolicy(t *testing.T) {
	useLocal(t)
	root := t.TempDir()
	sub := filepath.Join(root, "data")
	require.NoError(t, os.Mkdir(sub, 0o755))
	fakeHome(t, "/home/fixtureuser")

	m := func(typ MountType, src string) RunOption {
		return WithRunMount(RunMount{Type: typ, Source: src, Target: "/x"})
	}
	roots := WithRunMountAllowedRoots(root)

	// allowed
	args := allowed(t, "bind in root", roots, WithRunMount(RunMount{Type: MountBind, Source: sub, Target: "/mnt/d", ReadOnly: true}))
	i := indexOf(args, "--mount")
	require.GreaterOrEqual(t, i, 0)
	rs, _ := filepath.EvalSymlinks(sub)
	assert.Equal(t, "type=bind,source="+rs+",target=/mnt/d,readonly", args[i+1])
	assert.Equal(t, -1, indexOf(args, "-v"))
	allowed(t, "root itself", roots, m(MountBind, root))
	allowed(t, "named volume", m(MountVolume, "myvol"))
	allowed(t, "anonymous volume", m(MountVolume, ""))
	allowed(t, "tmpfs", WithRunMount(RunMount{Type: MountTmpfs, Target: "/scratch"}))
	allowed(t, "volume spec", roots, WithRunVolumes(sub+":/x:ro"))
	allowed(t, "volume spec named", WithRunVolumes("vol1:/x"))

	// refused
	refused(t, "no roots", m(MountBind, sub))
	refused(t, "empty type", roots, m("", sub))
	refused(t, "glob type", roots, m("glob", sub))
	refused(t, "device type", roots, m("devpts", sub))
	refused(t, "image type", roots, m("image", sub))
	refused(t, "outside roots", WithRunMountAllowedRoots(sub), m(MountBind, root))
	require.NoError(t, os.Mkdir(sub+"x", 0o755))
	refused(t, "sibling prefix", WithRunMountAllowedRoots(sub), m(MountBind, sub+"x"))
	refused(t, "relative", roots, m(MountBind, "data"))
	refused(t, "unclean", roots, m(MountBind, sub+"/../data"))
	refused(t, "comma injection", roots, m(MountBind, sub+",readonly=false"))
	refused(t, "quote injection", roots, m(MountBind, sub+`"`))
	refused(t, "named volume with slash", m(MountVolume, "a/b"))
	refused(t, "named volume dotdot", m(MountVolume, ".."))
	invalidOpt(t, "tmpfs source", WithRunMount(RunMount{Type: MountTmpfs, Source: "/x", Target: "/y"}))
	invalidOpt(t, "target relative", m2(MountVolume, "v", "rel"))
	invalidOpt(t, "target proc", m2(MountVolume, "v", "/proc/x"))
	invalidOpt(t, "target root", m2(MountVolume, "v", "/"))
	invalidOpt(t, "volume opt Z", roots, WithRunVolumes(sub+":/x:Z"))
	invalidOpt(t, "volume 4 parts", roots, WithRunVolumes(sub+":/x:ro:z"))
	refused(t, "volume spec relative src", roots, WithRunVolumes("./data:/x"))
	refused(t, "volume spec tilde", roots, WithRunVolumes("~/data:/x"))
	// denied roots
	for _, bad := range []string{"/", "/usr", "/usr/share", "/var", "/var/lib/docker", "/var/lib/containers/storage",
		"/home", "/home/fixtureuser", "/home/otheruser", "/etc", "/etc/ssl", "/boot", "/root", "/opt", "/srv",
		"/lib", "/lib64", "/bin", "/sbin", "/proc", "/sys", "/dev", "/run", "/var/run", "/run/user/1000", "/tmp"} {
		refused(t, "denied root "+bad, WithRunMountAllowedRoots(bad), m(MountBind, bad))
	}
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh", "k"), 0o755))
	fakeHome(t, home)
	refused(t, "home credential dir", WithRunMountAllowedRoots(filepath.Join(home, ".ssh")), m(MountBind, filepath.Join(home, ".ssh", "k")))
	refused(t, "ancestor of home", WithRunMountAllowedRoots(filepath.Dir(home)), m(MountBind, filepath.Dir(home)))
}

func m2(typ MountType, src, tgt string) RunOption {
	return WithRunMount(RunMount{Type: typ, Source: src, Target: tgt})
}

func TestAL_Mounts_SymlinkSocketAndHomeLocal(t *testing.T) {
	useLocal(t)
	root := t.TempDir()
	fakeHome(t, root) // the "home" is the root itself: must be denied even as an allowed root
	refused(t, "home as root", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: root, Target: "/x"}))
	fakeHome(t, "/home/fixtureuser")

	link := filepath.Join(root, "to-proc")
	require.NoError(t, os.Symlink("/proc", link))
	refused(t, "symlink to /proc", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: link, Target: "/x"}))
	link2 := filepath.Join(root, "to-etc")
	require.NoError(t, os.Symlink("/etc", link2))
	refused(t, "symlink to /etc", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: link2, Target: "/x"}))
	// symlink to a sibling dir outside the roots
	out := t.TempDir()
	link3 := filepath.Join(root, "to-out")
	require.NoError(t, os.Symlink(out, link3))
	refused(t, "symlink outside roots", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: link3, Target: "/x"}))
	// non-existent
	refused(t, "missing", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(root, "nope"), Target: "/x"}))
	// a real unix socket without a .sock name
	sockDir, derr := os.MkdirTemp("", "s")
	require.NoError(t, derr)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	ln, lerr := net.Listen("unix", filepath.Join(sockDir, "plain"))
	if lerr != nil {
		// never a silent pass: the real-socket leg did not run
		t.Skipf("real-socket leg NOT exercised, unix listen unavailable: %v", lerr)
	}
	defer ln.Close()
	refused(t, "real socket, plain name", WithRunMountAllowedRoots(sockDir),
		WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(sockDir, "plain"), Target: "/x"}))
	// *.sock by name
	refused(t, "sock name", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(root, "a.sock"), Target: "/x"}))
	// runtime state dirs in /tmp
	refused(t, "storage-run", WithRunMountAllowedRoots("/tmp/storage-run-1000"), WithRunMount(RunMount{Type: MountBind, Source: "/tmp/storage-run-1000/x", Target: "/x"}))
}

func TestAL_Mounts_RemoteIsLiteralNoLocalResolution(t *testing.T) {
	fakeHome(t, "/home/fixtureuser")
	// ResolveRunSpec (remote semantics): a path that does not exist locally is fine lexically
	args := allowed(t, "remote literal",
		WithRunMountAllowedRoots("/srv2/data"),
		WithRunMount(RunMount{Type: MountBind, Source: "/srv2/data/a", Target: "/x"}))
	assert.Contains(t, strings.Join(args, " "), "source=/srv2/data/a,")
	// lexical denies still apply on the remote side
	refused(t, "remote /etc", WithRunMountAllowedRoots("/etc"), WithRunMount(RunMount{Type: MountBind, Source: "/etc/ssl", Target: "/x"}))
	refused(t, "remote /usr", WithRunMountAllowedRoots("/srv2"), WithRunMount(RunMount{Type: MountBind, Source: "/usr/bin", Target: "/x"}))
	refused(t, "remote relative", WithRunMountAllowedRoots("/srv2"), WithRunMount(RunMount{Type: MountBind, Source: "srv2/a", Target: "/x"}))
	refused(t, "remote unclean", WithRunMountAllowedRoots("/srv2"), WithRunMount(RunMount{Type: MountBind, Source: "/srv2/../etc", Target: "/x"}))
	// local symlink in a local dir is NOT resolved remotely (documented): literal path accepted
	root := t.TempDir()
	link := filepath.Join(root, "l")
	require.NoError(t, os.Symlink("/proc", link))
	allowed(t, "remote does not stat locally", WithRunMountAllowedRoots(root), WithRunMount(RunMount{Type: MountBind, Source: link, Target: "/x"}))
}

func TestAL_Mounts_ResolvedOnlyDenyAndRemoteLexicalDenies(t *testing.T) {
	useLocal(t)
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "containers", "d"), 0o755))
	fakeHome(t, home)
	cfg := filepath.Join(home, ".config")
	link := filepath.Join(cfg, "lnk")
	require.NoError(t, os.Symlink(filepath.Join(cfg, "containers", "d"), link))
	// root is not denied, the lexical source is not denied, ONLY the resolved path is
	refused(t, "resolved-only deny", WithRunMountAllowedRoots(cfg),
		WithRunMount(RunMount{Type: MountBind, Source: link, Target: "/x"}))
	allowed(t, "control: plain dir under same root", WithRunMountAllowedRoots(cfg),
		WithRunMount(RunMount{Type: MountBind, Source: cfg, Target: "/x"}))

	localMode = false // ResolveRunSpec semantics: lexical only, no home known
	for _, src := range []string{"/home", "/home/someone", "/srv2/a.sock", "/srv2/b.socket", "/tmp", "/lib64/x", "/var/lib/docker"} {
		refused(t, "remote "+src, WithRunMountAllowedRoots("/srv2", src),
			WithRunMount(RunMount{Type: MountBind, Source: src, Target: "/x"}))
	}
	for _, src := range []string{"/home", "/home/someone", "/srv2/a.sock"} {
		refused(t, "remote src only "+src, WithRunMountAllowedRoots("/"+strings.Split(src, "/")[1], "/srv2"),
			WithRunMount(RunMount{Type: MountBind, Source: src, Target: "/x"}))
	}
}

func TestAL_Env_Name_Workdir_Validation(t *testing.T) {
	invalidOpt(t, "env name", WithRunEnv(map[string]string{"A=B": "x"}))
	invalidOpt(t, "env empty name", WithRunEnv(map[string]string{"": "x"}))
	invalidOpt(t, "container name", WithRunName("--privileged"))
	invalidOpt(t, "workdir rel", WithRunWorkDir("rel"))
}

// ---- live rootless podman ---------------------------------------------------

type countExec struct {
	*defaultExecutor
	n atomic.Int32
}

func (c *countExec) ExecuteWithStderr(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	c.n.Add(1)
	return c.defaultExecutor.ExecuteWithStderr(ctx, name, args...)
}

func TestAL_RealPodman_TypedMountWorks_GlobNeverReachesEngine(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("SKIP-OK: podman not present")
	}
	if err := exec.Command("podman", "image", "exists", "alpine:3.20").Run(); err != nil {
		t.Skip("SKIP-OK: alpine:3.20 not local, no pulls allowed")
	}
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello-typed-mount\n"), 0o644))
	require.NoError(t, os.Chmod(root, 0o755))
	ce := &countExec{defaultExecutor: &defaultExecutor{}}
	p := NewPodmanRuntimeWithExecutor(ce)
	ctx := context.Background()

	res, err := p.Run(ctx, "alpine:3.20", []string{"sh", "-c", "cat /mnt/d/f.txt; touch /mnt/d/w 2>&1 || echo ro-ok"},
		WithRunMountAllowedRoots(root),
		WithRunMount(RunMount{Type: MountBind, Source: root, Target: "/mnt/d", ReadOnly: true}),
		WithRunExtraArgs("--pull=never"))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "hello-typed-mount")
	assert.Contains(t, res.Stdout, "ro-ok", "read-only mount must reject writes: %s", res.Stdout)
	assert.Equal(t, int32(1), ce.n.Load())

	before := ce.n.Load()
	for _, opts := range [][]RunOption{
		{WithRunExtraArgs("--mount", "type=glob,src=/etc/*,target=/x")},
		{WithRunMount(RunMount{Type: "glob", Source: "/etc/*", Target: "/x"}), WithRunMountAllowedRoots("/etc")},
		{WithRunExtraArgs("--mount", "type=volume,volume-opt=device=/,target=/x")},
		{WithRunExtraArgs("--rootfs", "/")},
	} {
		_, err := p.Run(ctx, "alpine:3.20", []string{"true"}, opts...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrRunForbiddenOption), "%v", err)
	}
	assert.Equal(t, before, ce.n.Load(), "refused runs must never reach the engine")
}

func TestAL_RealPodman_CapabilityBitsAreWhatWeThink(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("SKIP-OK: podman not present")
	}
	if err := exec.Command("podman", "image", "exists", "alpine:3.20").Run(); err != nil {
		t.Skip("SKIP-OK: alpine:3.20 not local, no pulls allowed")
	}
	p := NewPodmanRuntime()
	res, err := p.Run(context.Background(), "alpine:3.20",
		[]string{"sh", "-c", "grep CapEff /proc/self/status"},
		WithRunCapDropAll(), WithRunCapAddReason("live check of NET_ADMIN bit", "NET_ADMIN"),
		WithRunNoNewPrivileges(), WithRunExtraArgs("--pull=never"))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	f := strings.Fields(res.Stdout)
	require.Len(t, f, 2, res.Stdout)
	var eff uint64
	for _, c := range f[1] {
		eff = eff<<4 | uint64(strings.IndexRune("0123456789abcdef", c))
	}
	assert.NotZero(t, eff&(1<<12), "NET_ADMIN (bit 12) granted")
	assert.Zero(t, eff&(1<<21), "SYS_ADMIN (bit 21) must NOT be granted")
	assert.Zero(t, eff&(1<<0), "CHOWN dropped by cap-drop=ALL")
}

// ---- round 4 ----

func TestR4_SymlinkedAllowedRootToHomeIsRedenied(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	require.NoError(t, os.Mkdir(home, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".bashrc"), []byte("x"), 0o644))
	fakeHome(t, home)
	link := filepath.Join(base, "h")
	require.NoError(t, os.Symlink(home, link))
	refused(t, "root symlink to home", WithRunMountAllowedRoots(link),
		WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(link, ".bashrc"), Target: "/x"}))
}

func TestR4_CommaQuoteNewlineInjection(t *testing.T) {
	useLocal(t)
	root := t.TempDir()
	comma := filepath.Join(root, "a,b")
	require.NoError(t, os.Mkdir(comma, 0o755))
	quote := filepath.Join(root, `q"x`)
	require.NoError(t, os.Mkdir(quote, 0o755))
	nl := filepath.Join(root, "n\nl")
	require.NoError(t, os.Mkdir(nl, 0o755))
	ok := filepath.Join(root, "ok")
	require.NoError(t, os.Mkdir(ok, 0o755))
	roots := WithRunMountAllowedRoots(root)
	for _, src := range []string{comma, quote, nl} {
		refused(t, "source "+src, roots, WithRunMount(RunMount{Type: MountBind, Source: src, Target: "/x"}))
	}
	for _, tgt := range []string{"/a,b", `/a"b`, "/a\nb"} {
		spec := resolveSpec([]RunOption{roots, WithRunMount(RunMount{Type: MountBind, Source: ok, Target: tgt})})
		require.Error(t, spec.Err, tgt)
		spec = resolveSpec([]RunOption{WithRunMount(RunMount{Type: MountTmpfs, Target: tgt})})
		require.Error(t, spec.Err, "tmpfs "+tgt)
		spec = resolveSpec([]RunOption{extra("--tmpfs", tgt)})
		require.Error(t, spec.Err, "--tmpfs "+tgt)
	}
}

func TestR4_AddHostAndUlimitNames(t *testing.T) {
	for _, v := range []string{"bad_host!:10.0.0.1", "-x:1.1.1.1", ":1.1.1.1", "a b:1.1.1.1", "h:", "h:1.1.1"} {
		invalidOpt(t, "add-host "+v, extra("--add-host", v))
	}
	allowed(t, "add-host ok", extra("--add-host", "a.b-c:::1"))
	for _, v := range []string{"memlock=1", "rtprio=1", "nice=1", "locks=1", "bogus=1", "nofile", "=1", "nofile=1:2:3"} {
		invalidOpt(t, "ulimit "+v, extra("--ulimit", v))
	}
	for _, n := range []string{"nofile", "nproc", "core", "cpu", "fsize", "stack"} {
		allowed(t, "ulimit "+n, extra("--ulimit", n+"=10"))
	}
}

func TestR4_StopSignalRange(t *testing.T) {
	for _, v := range []string{"0", "65", "-1", "99999", "sig term", "term!"} {
		invalidOpt(t, "stop-signal "+v, extra("--stop-signal", v))
	}
	for _, v := range []string{"1", "64", "SIGTERM", "TERM", "KILL"} {
		allowed(t, "stop-signal "+v, extra("--stop-signal", v))
	}
}

func TestR4_ExtraHomeAndSystemDenies(t *testing.T) {
	useLocal(t)
	home := t.TempDir()
	fakeHome(t, home)
	require.NoError(t, os.Mkdir(filepath.Join(home, "w"), 0o755))
	for _, c := range []string{".bashrc", ".profile", ".config/systemd", ".config/autostart", ".local/bin", ".netrc", ".zshrc", ".bash_profile"} {
		d := filepath.Join(home, c)
		require.NoError(t, os.MkdirAll(d, 0o755))
		refused(t, "home "+c, WithRunMountAllowedRoots(filepath.Join(home, "w")), WithRunMount(RunMount{Type: MountBind, Source: d, Target: "/x"}))
	}
	localMode = false
	for _, p := range []string{"/mnt/x", "/media/x", "/snap/x"} {
		refused(t, p, WithRunMountAllowedRoots(p), WithRunMount(RunMount{Type: MountBind, Source: p, Target: "/x"}))
	}
}

func TestR4_ReasonTiedToCapability(t *testing.T) {
	spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{WithRunCapAddReason("needs tun for the test rig", "NET_ADMIN")})
	require.NoError(t, spec.Err)
	assert.Equal(t, []string{"NET_ADMIN: needs tun for the test rig"}, spec.Reasons)
	// a reason given for one cap does not license NET_ADMIN added separately
	refused(t, "reason for other cap", WithRunCapAddReason("a perfectly long reason", "CHOWN"), WithRunCapAdd("NET_ADMIN"))
}

// The shape llmctl/internal/vantage passes today, with typed equivalents.
func TestR4_VantageShapeTypedEquivalent(t *testing.T) {
	useLocal(t)
	probe := t.TempDir()
	mem, err := ParseRunMemory("128m")
	require.NoError(t, err)
	assert.Equal(t, int64(128<<20), mem)
	args := allowed(t, "vantage typed",
		WithRunName("vantage-ab12"), WithRunRemove(true), WithRunUser("0:0"),
		WithRunEntrypoint("/vantage/vantage-probe"), WithRunNetwork("bridge"),
		WithRunMountAllowedRoots(probe), WithRunVolumes(probe+":/vantage:ro"),
		WithRunDetach(), WithRunMemory(mem), WithRunMemorySwap(mem), WithRunPidsLimit(64),
		WithRunCapDropAll(), WithRunLabel("k1", "1"), WithRunLabel("k2", "owner"),
		WithRunExtraArgs("--pull=never", "--read-only"))
	assert.Contains(t, args, "-d")
	assert.Contains(t, args, "--cap-drop=ALL")
	assert.Contains(t, args, "--read-only")
	// the verbatim old shape is NOT accepted: the typed options are required
	refused(t, "verbatim old shape", WithRunMountAllowedRoots(probe), WithRunVolumes(probe+":/vantage:ro"),
		WithRunExtraArgs("-d", "--pull=never", "--memory", "128m", "--memory-swap", "128m",
			"--pids-limit", "64", "--cap-drop", "all", "--read-only"))
	// bounds
	for _, o := range []RunOption{WithRunPidsLimit(1 << 21), WithRunMemory(1 << 20), WithRunCPUs(1e9)} {
		invalidOpt(t, "bounds", o)
	}
	for _, s := range []string{"", "x", "-1", "1t", "128mb", "99999999999999999999"} {
		_, err := ParseRunMemory(s)
		assert.Error(t, err, s)
	}
}

// ---- round 5: survivors of the 69-mutant run ----

// M53: only the allowed-root re-denial after symlink resolution can refuse
// this call (the source is an ordinary, non-denied file below the home).
func TestR5_SymlinkedRootResolvingToHomeRefusedByRootCheck(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "work"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, "work", "f"), []byte("x"), 0o644))
	fakeHome(t, home)
	link := filepath.Join(base, "h")
	require.NoError(t, os.Symlink(home, link))
	spec := resolveSpec([]RunOption{WithRunMountAllowedRoots(link),
		WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(link, "work", "f"), Target: "/x"})})
	require.Error(t, spec.Err)
	assert.True(t, errors.Is(spec.Err, ErrRunForbiddenOption), "%v", spec.Err)
	assert.Contains(t, spec.Err.Error(), "resolves to a denied path")
}

// M66: every home child on the deny list is denied directly (the R4 loop
// above is refused by the allowed-root check first and cannot isolate it).
func TestR5_DenyHostPathCoversEveryHomeChild(t *testing.T) {
	home := "/home/zz-user"
	for _, c := range denyHomeChildren {
		d := filepath.Join(home, c)
		for _, p := range []string{d, d + "/sub"} {
			denied, _ := denyHostPath(p, home)
			assert.True(t, denied, p)
		}
	}
	for _, c := range []string{".ssh", ".bashrc", ".bash_profile", ".profile", ".zshrc", ".netrc", ".config/systemd", ".local/bin"} {
		denied, _ := denyHostPath(filepath.Join(home, c), home)
		assert.True(t, denied, c)
	}
	denied, _ := denyHostPath(filepath.Join(home, "work", "f"), home)
	assert.False(t, denied)
}

// M68: each memory unit has its exact binary multiplier.
func TestR5_ParseRunMemoryUnits(t *testing.T) {
	for in, want := range map[string]int64{"4096": 4096, "7b": 7, "1k": 1 << 10, "3K": 3 << 10, "1m": 1 << 20, "1g": 1 << 30, "2G": 2 << 30} {
		got, err := ParseRunMemory(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
}

// ---- round 5 (reviewer round 4): symlinked HOME and deny-list members ----

func bm5(src string) RunOption {
	return WithRunMount(RunMount{Type: MountBind, Source: src, Target: "/x"})
}

// F1: HOME is itself a symlink; the caller names the REAL path. Every
// home/credential deny must hold in both lexical and resolved form.
func TestR5_SymlinkedHomeDoesNotBypassHomeDenies(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	real := filepath.Join(base, "data", "u")
	for _, d := range []string{".ssh", "work", ".config/gh", ".local/share/keyrings"} {
		require.NoError(t, os.MkdirAll(filepath.Join(real, d), 0o700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(real, ".ssh", "id"), []byte("k"), 0o600))
	link := filepath.Join(base, "homelink")
	require.NoError(t, os.Symlink(real, link))

	// control: HOME = real path is refused
	fakeHome(t, real)
	refused(t, "ctrl real home root", WithRunMountAllowedRoots(real), bm5(filepath.Join(real, "work")))
	// control: ordinary work dir is accepted
	allowed(t, "ctrl work", WithRunMountAllowedRoots(filepath.Join(real, "work")), bm5(filepath.Join(real, "work")))

	fakeHome(t, link)
	refused(t, "symlinked HOME, root=real home", WithRunMountAllowedRoots(real), bm5(filepath.Join(real, "work")))
	refused(t, "symlinked HOME, root=real .ssh", WithRunMountAllowedRoots(filepath.Join(real, ".ssh")), bm5(filepath.Join(real, ".ssh", "id")))
	refused(t, "symlinked HOME, root=real .config/gh", WithRunMountAllowedRoots(filepath.Join(real, ".config")), bm5(filepath.Join(real, ".config", "gh")))
	refused(t, "symlinked HOME, source=real .ssh under work root", WithRunMountAllowedRoots(filepath.Join(real, "work")), bm5(filepath.Join(real, ".ssh")))
	allowed(t, "symlinked HOME, work still fine", WithRunMountAllowedRoots(filepath.Join(real, "work")), bm5(filepath.Join(real, "work")))
	// a symlink at an intermediate component of the home path
	mid := filepath.Join(base, "mid")
	require.NoError(t, os.Symlink(filepath.Join(base, "data"), mid))
	fakeHome(t, filepath.Join(mid, "u"))
	refused(t, "intermediate-symlink HOME", WithRunMountAllowedRoots(filepath.Join(real, ".ssh")), bm5(filepath.Join(real, ".ssh", "id")))
}

// F2: every /tmp runtime-state prefix is denied (lexical, direct call).
func TestR5_TmpStatePrefixesDenied(t *testing.T) {
	for _, n := range []string{"storage-run-1", "podman-run-1", "containers-user-1", "podman-x", "tmux-1000", "ssh-abc", ".X11-unix", "systemd-private-x"} {
		d, _ := denyHostPath("/tmp/"+n+"/s", "")
		assert.True(t, d, n)
	}
	d, _ := denyHostPath("/tmp/ordinary/s", "")
	assert.False(t, d)
}

// F4: more credential homes.
func TestR5_MoreHomeCredentialChildrenDenied(t *testing.T) {
	for _, c := range []string{".config/gh", ".git-credentials", ".local/share/keyrings", ".pgpass"} {
		d, _ := denyHostPath("/home/zz-user/"+c, "/home/zz-user")
		assert.True(t, d, c)
	}
}

// F5: CR and TAB in a container target.
func TestR5_TargetRejectsCRAndTab(t *testing.T) {
	for _, tg := range []string{"/x\r", "/x\ty", "/x\n"} {
		assert.Error(t, containerTarget(tg), "%q", tg)
	}
	assert.NoError(t, containerTarget("/x/y"))
}

// anchorForms returns the lexical and the symlink-resolved form of an anchor.
func TestR5_AnchorFormsResolvesSymlinks(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	require.NoError(t, os.Mkdir(real, 0o755))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(real, link))
	rr, err := filepath.EvalSymlinks(real)
	require.NoError(t, err)
	assert.Equal(t, []string{link, rr}, anchorForms(link, true))
	assert.Equal(t, []string{link}, anchorForms(link, false))
}
