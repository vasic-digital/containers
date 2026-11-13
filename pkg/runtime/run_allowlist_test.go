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
	if ln, lerr := net.Listen("unix", filepath.Join(sockDir, "plain")); lerr == nil {
		defer ln.Close()
		refused(t, "real socket, plain name", WithRunMountAllowedRoots(sockDir),
			WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(sockDir, "plain"), Target: "/x"}))
	} else {
		t.Logf("socket listen unavailable: %v", lerr)
	}
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
