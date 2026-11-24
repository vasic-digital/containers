package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCDS_R3_ArgvResolved(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	real := filepath.Join(base, "data", "real")
	_ = os.MkdirAll(real, 0o700)
	_ = os.MkdirAll(filepath.Join(base, "w"), 0o700)
	_ = os.Symlink("../data/./real/", filepath.Join(base, "w", "lnk")) // relative, dot, trailing slash
	spec := resolveSpec([]RunOption{WithRunMountAllowedRoots(base), WithRunMount(RunMount{Type: MountBind, Source: filepath.Join(base, "w", "lnk"), Target: "/x"})})
	if spec.Err != nil {
		t.Fatal(spec.Err)
	}
	j := strings.Join(spec.Args, " ")
	t.Logf("argv: %s", j)
	if !strings.Contains(j, "source="+real+",") || strings.Contains(j, "lnk") {
		t.Errorf("argv does not carry the resolved path")
	}
	// /proc/self/root alias
	for _, s := range []string{"/proc/self/root" + base + "/w", "/proc/1/root" + base + "/w"} {
		sp := resolveSpec([]RunOption{WithRunMountAllowedRoots("/proc/self/root" + base), WithRunMount(RunMount{Type: MountBind, Source: s, Target: "/x"})})
		t.Logf("%s err=%v", s, sp.Err)
		if sp.Err == nil {
			t.Errorf("%s accepted", s)
		}
	}
}
