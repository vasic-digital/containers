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

// Round-2 review fixes (F1-F5) for the G1 run builder. PROVISIONAL.

func refused(t *testing.T, name string, opts ...RunOption) {
	t.Helper()
	spec := ResolveRunSpec("alpine:3.20", nil, opts)
	require.Error(t, spec.Err, name)
	assert.True(t, errors.Is(spec.Err, ErrRunForbiddenOption), "%s: %v", name, spec.Err)
	assert.Nil(t, spec.Args, name)
}

func allowed(t *testing.T, name string, opts ...RunOption) {
	t.Helper()
	spec := ResolveRunSpec("alpine:3.20", nil, opts)
	assert.NoError(t, spec.Err, name)
}

func TestR2_F1_CapAddCommaListsAndPrefixes(t *testing.T) {
	for _, v := range []string{
		"NET_ADMIN,SYS_ADMIN", "net_admin,sys_admin", "CAP_SYS_ADMIN", "cap_sys_admin",
		"NET_ADMIN,CAP_SYS_ADMIN", "chown,all", "ALL", "SYS_MODULE", "SYS_RAWIO", "SYS_PTRACE",
		" NET_ADMIN , SYS_ADMIN ",
	} {
		refused(t, "cap-add="+v, WithRunExtraArgs("--cap-add="+v))
		refused(t, "cap-add "+v, WithRunExtraArgs("--cap-add", v))
	}
	// control: harmless capabilities stay allowed
	allowed(t, "NET_ADMIN", WithRunExtraArgs("--cap-add=NET_ADMIN"))
	allowed(t, "NET_BIND_SERVICE,CHOWN", WithRunExtraArgs("--cap-add", "NET_BIND_SERVICE,CHOWN"))
}

func TestR2_F1_RealPodman_CapabilityBitsAreWhatWeThink(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("SKIP-OK: podman not present")
	}
	if err := exec.Command("podman", "image", "exists", "alpine:3.20").Run(); err != nil {
		t.Skip("SKIP-OK: alpine:3.20 not local, no pulls allowed")
	}
	p := NewPodmanRuntime()
	// the bypass shape is refused BEFORE any container exists
	_, err := p.Run(context.Background(), "alpine:3.20", []string{"true"},
		WithRunExtraArgs("--cap-add=NET_ADMIN,SYS_ADMIN"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRunForbiddenOption), "%v", err)
	// control: an allowed cap really is granted, and SYS_ADMIN (bit 21) is not
	res, err := p.Run(context.Background(), "alpine:3.20",
		[]string{"sh", "-c", "grep CapEff /proc/self/status"},
		WithRunExtraArgs("--cap-add=NET_ADMIN"))
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
}

func TestR2_F2_NamespacePathsAndContainerJoins(t *testing.T) {
	for _, fl := range []string{"--network", "--net", "--pid", "--ipc", "--uts", "--cgroupns", "--userns"} {
		for _, v := range []string{"ns:/proc/1/ns/net", "NS:/proc/1/ns/net", "container:other", "Container:x"} {
			refused(t, fl+"="+v, WithRunExtraArgs(fl+"="+v))
			refused(t, fl+" "+v, WithRunExtraArgs(fl, v))
		}
	}
	refused(t, "typed ns", WithRunNetwork("ns:/proc/1/ns/net"))
	refused(t, "typed container", WithRunNetwork("container:other"))
	refused(t, "slirp host loopback", WithRunNetwork("slirp4netns:allow_host_loopback=true"))
	// controls
	allowed(t, "bridge named", WithRunNetwork("mybridge"))
	allowed(t, "pasta", WithRunExtraArgs("--network=pasta"))
	allowed(t, "keep-id extra", WithRunExtraArgs("--userns=keep-id"))
}

func TestR2_F3_DeviceOnlyThroughTypedOption(t *testing.T) {
	refused(t, "--device", WithRunExtraArgs("--device", "/dev/mem"))
	refused(t, "--device=", WithRunExtraArgs("--device=/dev/kvm"))
	refused(t, "--device-cgroup-rule", WithRunExtraArgs("--device-cgroup-rule=a *:* rwm"))
	refused(t, "--device-cgroup-rule sp", WithRunExtraArgs("--device-cgroup-rule", "a *:* rwm"))
}

func TestR2_F4_BindMountSources(t *testing.T) {
	bad := []string{
		"/", "/proc", "/proc/1/root", "/sys", "/sys/kernel", "/dev", "/dev/mem", "/run",
		"/run/user/1000/podman/podman.sock", "/var/run/docker.sock", "/var/run", "/etc", "/boot", "/root", "/home",
		"/srv/anywhere/docker.sock", "/tmp/x/podman.sock", "/proc/", "//", "/etc/../proc", "/home/",
	}
	for _, src := range bad {
		refused(t, "-v "+src, WithRunVolumes(src+":/x"))
		refused(t, "-v "+src+":ro", WithRunVolumes(src+":/x:ro"))
		refused(t, "extra -v "+src, WithRunExtraArgs("-v", src+":/x"))
		refused(t, "extra -v= "+src, WithRunExtraArgs("-v="+src+":/x"))
		refused(t, "extra -vSRC "+src, WithRunExtraArgs("-v"+src+":/x"))
		refused(t, "extra --volume "+src, WithRunExtraArgs("--volume", src+":/x"))
		refused(t, "extra --volume= "+src, WithRunExtraArgs("--volume="+src+":/x"))
		refused(t, "--mount source "+src, WithRunExtraArgs("--mount", "type=bind,source="+src+",target=/x"))
		refused(t, "--mount= src "+src, WithRunExtraArgs("--mount=type=bind,src="+src+",target=/x,ro"))
	}
	good := []string{"/home/someone/project", "/tmp/work", "/etc/ssl/certs", "/root/.cache", "/srv/data", "/var/lib/x", "relative-named-volume"}
	for _, src := range good {
		allowed(t, "-v "+src, WithRunVolumes(src+":/x"))
		allowed(t, "extra --mount "+src, WithRunExtraArgs("--mount", "type=bind,source="+src+",target=/x"))
	}
	// non-bind mounts are not bind mounts
	allowed(t, "tmpfs mount", WithRunExtraArgs("--mount", "type=tmpfs,target=/x"))
	allowed(t, "volume mount", WithRunExtraArgs("--mount", "type=volume,source=/proc,target=/x"))
}

func TestR2_F4_SymlinkResolvesToProtectedSource(t *testing.T) {
	dir := t.TempDir()
	link := dir + "/to-proc"
	require.NoError(t, osSymlink("/proc", link))
	refused(t, "symlink to /proc", WithRunVolumes(link+":/x"))
}

func TestR2_F5_SecurityOptColonAndEqualsForms(t *testing.T) {
	for _, v := range []string{"label=disable", "label:disable", "LABEL:DISABLE", "seccomp=unconfined", "apparmor=unconfined", "systempaths=unconfined", "seccomp:unconfined"} {
		refused(t, "--security-opt="+v, WithRunExtraArgs("--security-opt="+v))
		refused(t, "--security-opt "+v, WithRunExtraArgs("--security-opt", v))
	}
	allowed(t, "no-new-privileges", WithRunExtraArgs("--security-opt=no-new-privileges"))
	allowed(t, "label=type:x", WithRunExtraArgs("--security-opt=label=type:container_t"))
}
