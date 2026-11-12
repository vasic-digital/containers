//go:build !integration

package runtime

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// G1 (CAT-DEVEMU): typed resource / device / publish / label / digest options
// for the one-shot run primitive. PROVISIONAL: unit + one real rootless podman
// cgroup read-back; nothing here proves an emulator boots.

const goodDigest = "alpine@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func argsOf(t *testing.T, opts ...RunOption) []string {
	t.Helper()
	spec := ResolveRunSpec("alpine:3.20", []string{"true"}, opts)
	require.NoError(t, spec.Err)
	return spec.Args
}

func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

func flagValue(args []string, flag string) (string, bool) {
	i := indexOf(args, flag)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

func TestG1_MemoryDefaultsSwapToMemory(t *testing.T) {
	args := argsOf(t, WithRunMemory(512<<20))
	v, ok := flagValue(args, "--memory")
	require.True(t, ok, "argv=%v", args)
	assert.Equal(t, "536870912", v)
	s, ok := flagValue(args, "--memory-swap")
	require.True(t, ok, "swap must default to memory (no swap): %v", args)
	assert.Equal(t, "536870912", s)
}

func TestG1_ExplicitSwapAndValidation(t *testing.T) {
	args := argsOf(t, WithRunMemory(512<<20), WithRunMemorySwap(1<<30))
	s, _ := flagValue(args, "--memory-swap")
	assert.Equal(t, "1073741824", s)

	for name, opts := range map[string][]RunOption{
		"zero memory":       {WithRunMemory(0)},
		"negative memory":   {WithRunMemory(-1)},
		"swap without mem":  {WithRunMemorySwap(1 << 30)},
		"swap below memory": {WithRunMemory(1 << 30), WithRunMemorySwap(1 << 20)},
		"zero cpus":         {WithRunCPUs(0)},
		"negative cpus":     {WithRunCPUs(-2)},
		"zero pids":         {WithRunPidsLimit(0)},
		"unlimited pids -1": {WithRunPidsLimit(-1)},
	} {
		spec := ResolveRunSpec("alpine:3.20", nil, opts)
		assert.Error(t, spec.Err, name)
	}
}

func TestG1_CPUsAndPids(t *testing.T) {
	args := argsOf(t, WithRunCPUs(1.5), WithRunPidsLimit(256))
	c, _ := flagValue(args, "--cpus")
	assert.Equal(t, "1.5", c)
	p, _ := flagValue(args, "--pids-limit")
	assert.Equal(t, "256", p)
}

func TestG1_NoLimitFlagsByDefault(t *testing.T) {
	args := argsOf(t)
	for _, f := range []string{"--memory", "--memory-swap", "--cpus", "--pids-limit", "--device", "--userns", "-p", "--label"} {
		assert.Equal(t, -1, indexOf(args, f), "%s must be absent by default: %v", f, args)
	}
}

func TestG1_PrivilegedAndHostNetworkRefused_ClassWide(t *testing.T) {
	// every spelling of the two forbidden escalations, through every entry point
	cases := map[string][]RunOption{
		"extra --privileged":              {WithRunExtraArgs("--privileged")},
		"extra --privileged=true":         {WithRunExtraArgs("--privileged=true")},
		"extra --network=host":            {WithRunExtraArgs("--network=host")},
		"extra --network host":            {WithRunExtraArgs("--network", "host")},
		"extra --net=host":                {WithRunExtraArgs("--net=host")},
		"extra --net host":                {WithRunExtraArgs("--net", "host")},
		"extra --network=HOST":            {WithRunExtraArgs("--network=HOST")},
		"WithRunNetwork host":             {WithRunNetwork("host")},
		"WithRunNetwork Host":             {WithRunNetwork("Host")},
		"extra --userns=host":             {WithRunExtraArgs("--userns=host")},
		"extra --cap-add=ALL":             {WithRunExtraArgs("--cap-add=ALL")},
		"extra --security-opt unconfined": {WithRunExtraArgs("--security-opt=seccomp=unconfined")},
	}
	for name, opts := range cases {
		spec := ResolveRunSpec("alpine:3.20", nil, opts)
		require.Error(t, spec.Err, name)
		assert.True(t, errors.Is(spec.Err, ErrRunForbiddenOption), "%s: %v", name, spec.Err)
		assert.Nil(t, spec.Args, "%s: no argv may be produced for a refused run", name)
	}
	// controls: benign spellings are NOT refused (false-positive guard)
	for name, opts := range map[string][]RunOption{
		"network none":          {WithRunNetwork("none")},
		"read-only extra":       {WithRunExtraArgs("--read-only")},
		"network named host-x":  {WithRunNetwork("hostnet")},
		"extra --network=pasta": {WithRunExtraArgs("--network=pasta")},
	} {
		spec := ResolveRunSpec("alpine:3.20", nil, opts)
		assert.NoError(t, spec.Err, name)
	}
}

func TestG1_ImageLeadingDashRefused(t *testing.T) {
	spec := ResolveRunSpec("--privileged", nil, nil)
	require.Error(t, spec.Err)
	assert.True(t, errors.Is(spec.Err, ErrRunForbiddenOption))
}

func TestG1_Device_AllowlistAndShape(t *testing.T) {
	old := deviceUsable
	deviceUsable = func(string) error { return nil }
	t.Cleanup(func() { deviceUsable = old })

	args := argsOf(t, WithRunDevice("/dev/kvm", "/dev/kvm"))
	v, ok := flagValue(args, "--device")
	require.True(t, ok)
	assert.Equal(t, "/dev/kvm", v)

	for name, opt := range map[string]RunOption{
		"not allowlisted": WithRunDevice("/dev/kvm"),
		"other allowed":   WithRunDevice("/dev/kvm", "/dev/net/tun"),
		"relative":        WithRunDevice("kvm", "kvm"),
		"outside dev":     WithRunDevice("/etc/passwd", "/etc/passwd"),
		"traversal":       WithRunDevice("/dev/../etc/shadow", "/dev/../etc/shadow"),
		"colon injection": WithRunDevice("/dev/kvm:rwm", "/dev/kvm:rwm"),
		"dev root itself": WithRunDevice("/dev", "/dev"),
	} {
		spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{opt})
		if name == "other allowed" { // allowlist contains a different device than the one requested? it IS /dev/kvm requested with list [/dev/net/tun]? no: path=/dev/kvm allow=[/dev/net/tun]
			assert.Error(t, spec.Err, name)
			continue
		}
		assert.Error(t, spec.Err, name)
	}
}

func TestG1_Device_KVMMustBeOpenable(t *testing.T) {
	old := deviceUsable
	deviceUsable = func(p string) error { return errors.New("permission denied: " + p) }
	t.Cleanup(func() { deviceUsable = old })
	spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{WithRunDevice("/dev/kvm", "/dev/kvm")})
	require.Error(t, spec.Err)
	assert.Contains(t, spec.Err.Error(), "permission denied")
}

func TestG1_UsernsKeepIDPodmanOnly(t *testing.T) {
	spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{WithRunUserns("keep-id")})
	require.NoError(t, spec.Err) // runtime-agnostic resolve: no runtime known
	v := strings.Join(spec.Args, " ")
	assert.Contains(t, v, "--userns keep-id")

	// docker/nerdctl path refuses with a typed error
	exe := &stdinCapableExecutor{}
	_, err := runViaCLI(context.Background(), exe, "docker", "docker", "alpine:3.20", nil,
		[]RunOption{WithRunUserns("keep-id")})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRunUnsupportedOption), "%v", err)
	assert.Empty(t, exe.gotArgs, "executor must not be invoked")

	// podman accepts
	_, err = runViaCLI(context.Background(), exe, "podman", "podman", "alpine:3.20", nil,
		[]RunOption{WithRunUserns("keep-id")})
	require.NoError(t, err)

	// only keep-id is modelled
	spec = ResolveRunSpec("alpine:3.20", nil, []RunOption{WithRunUserns("host")})
	assert.Error(t, spec.Err)
}

func TestG1_Publish(t *testing.T) {
	args := argsOf(t, WithRunPublish("127.0.0.1", 6555, 5575, "tcp"))
	v, ok := flagValue(args, "-p")
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1:6555:5575/tcp", v)

	for name, opt := range map[string]RunOption{
		"empty host ip":  WithRunPublish("", 1, 2, "tcp"),
		"wildcard v4":    WithRunPublish("0.0.0.0", 1, 2, "tcp"),
		"wildcard v6":    WithRunPublish("::", 1, 2, "tcp"),
		"not an ip":      WithRunPublish("localhost", 1, 2, "tcp"),
		"bad proto":      WithRunPublish("127.0.0.1", 1, 2, "icmp"),
		"ctr port 0":     WithRunPublish("127.0.0.1", 1, 0, "tcp"),
		"host port high": WithRunPublish("127.0.0.1", 70000, 2, "tcp"),
		"injection":      WithRunPublish("127.0.0.1:1", 1, 2, "tcp"),
	} {
		spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{opt})
		assert.Error(t, spec.Err, name)
	}
	// explicit opt-in for all interfaces
	spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{
		WithRunPublish("0.0.0.0", 8080, 80, "tcp"), WithRunPublishAllInterfaces()})
	require.NoError(t, spec.Err)
	assert.Contains(t, spec.Args, "0.0.0.0:8080:80/tcp")
}

func TestG1_Labels_Sorted(t *testing.T) {
	args := argsOf(t, WithRunLabel("b", "2"), WithRunLabel("a", "1"))
	var got []string
	for i, a := range args {
		if a == "--label" {
			got = append(got, args[i+1])
		}
	}
	assert.Equal(t, []string{"a=1", "b=2"}, got)
	for _, k := range []string{"", "a=b"} {
		spec := ResolveRunSpec("alpine:3.20", nil, []RunOption{WithRunLabel(k, "v")})
		assert.Error(t, spec.Err, "key %q", k)
	}
}

func TestG1_ImageDigestPinning(t *testing.T) {
	spec := ResolveRunSpec(goodDigest, nil, []RunOption{WithRunImageDigest()})
	require.NoError(t, spec.Err)
	for _, bad := range []string{"alpine:3.20", "alpine:latest", "alpine@sha256:abc", "alpine@sha1:" + strings.Repeat("a", 40), "alpine:3.20@sha256:" + strings.Repeat("g", 64)} {
		spec := ResolveRunSpec(bad, nil, []RunOption{WithRunImageDigest()})
		assert.Error(t, spec.Err, bad)
	}
	// without the option a tag is fine
	assert.NoError(t, ResolveRunSpec("alpine:3.20", nil, nil).Err)
}

func TestG1_PodmanRunSurfacesOptionErrorWithoutExecuting(t *testing.T) {
	exe := &stdinCapableExecutor{}
	p := NewPodmanRuntimeWithExecutor(exe)
	_, err := p.Run(context.Background(), "alpine:3.20", nil, WithRunMemory(-5))
	require.Error(t, err)
	assert.Empty(t, exe.gotArgs)
	assert.Equal(t, "", exe.gotName)
}

func TestG1_ArgvOrderEndsWithImageAndCmd(t *testing.T) {
	args := argsOf(t, WithRunMemory(64<<20), WithRunCPUs(1), WithRunPidsLimit(64),
		WithRunLabel("k", "v"), WithRunPublish("127.0.0.1", 1, 2, "tcp"))
	n := len(args)
	assert.Equal(t, []string{"alpine:3.20", "true"}, args[n-2:])
}

// Real rootless podman: the limit is not just in argv, the cgroup says so.
func TestG1_RealPodman_MemoryLimitIsEnforcedInCgroup(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("SKIP-OK: podman not present (hardware_not_present equivalent); tracked CAT-DEVEMU")
	}
	if out, err := exec.Command("podman", "image", "exists", "alpine:3.20").CombinedOutput(); err != nil {
		t.Skipf("SKIP-OK: alpine:3.20 not local, no pulls allowed: %v %s", err, out)
	}
	p := NewPodmanRuntime()
	res, err := p.Run(context.Background(), "alpine:3.20",
		[]string{"sh", "-c", "cat /sys/fs/cgroup/memory.max /sys/fs/cgroup/memory.swap.max /sys/fs/cgroup/pids.max"},
		WithRunMemory(96<<20), WithRunPidsLimit(77), WithRunCPUs(1))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	lines := strings.Fields(res.Stdout)
	require.Len(t, lines, 3, res.Stdout)
	assert.Equal(t, "100663296", lines[0], "memory.max")
	assert.Equal(t, "0", lines[1], "memory.swap.max (swap == memory => no swap)")
	assert.Equal(t, "77", lines[2], "pids.max")
}
