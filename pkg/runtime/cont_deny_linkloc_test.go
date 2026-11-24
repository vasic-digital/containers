package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCDS_R3_LinkLocationAncestor2(t *testing.T) {
	useLocal(t)
	oldH, oldF := homeDir, fallbackHome
	t.Cleanup(func() { homeDir, fallbackHome = oldH, oldF })
	base := t.TempDir()
	mk := func(p string) {
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ln := func(o, n string) {
		if err := os.Symlink(o, n); err != nil {
			t.Fatal(err)
		}
	}
	// HOME symlinked; ~/.local/bin and ~/.local/share are symlinks to another disk
	real := filepath.Join(base, "realhome")
	mk(filepath.Join(real, ".local"))
	mk(filepath.Join(base, "disk", "bin"))
	mk(filepath.Join(base, "disk", "share"))
	ln(filepath.Join(base, "disk", "bin"), filepath.Join(real, ".local", "bin"))
	ln(filepath.Join(base, "disk", "share"), filepath.Join(real, ".local", "share"))
	hl := filepath.Join(base, "h")
	ln(real, hl)
	homeDir = func() string { return hl }
	fallbackHome = func() string { return hl }
	for _, c := range []string{filepath.Join(real, ".local"), filepath.Join(hl, ".local")} {
		_, err := bindSource(&runOptions{MountRoots: []string{c}}, c)
		t.Logf("mount %s err=%v", c, err)
		if err == nil {
			t.Errorf("BYPASS: %s accepted; container can replace the ~/.local/bin link (PATH hijack)", c)
		}
	}
	// Variant without symlinked HOME: HOME real, but reach .local via another link.
	real2 := filepath.Join(base, "r2")
	mk(filepath.Join(real2, ".local"))
	mk(filepath.Join(base, "disk2", "bin"))
	mk(filepath.Join(base, "disk2", "share"))
	ln(filepath.Join(base, "disk2", "bin"), filepath.Join(real2, ".local", "bin"))
	ln(filepath.Join(base, "disk2", "share"), filepath.Join(real2, ".local", "share"))
	homeDir = func() string { return real2 }
	fallbackHome = func() string { return real2 }
	mk(filepath.Join(base, "w"))
	ln(filepath.Join(real2, ".local"), filepath.Join(base, "w", "loc"))
	c := filepath.Join(base, "w", "loc")
	_, err := bindSource(&runOptions{MountRoots: []string{filepath.Join(base, "w")}}, c)
	t.Logf("variant2 mount %s err=%v", c, err)
	if err == nil {
		t.Errorf("variant2: %s accepted although it is a link to ~/.local (whose bin/share entries are links)", c)
	}
}
