package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCDS_R3_HopBoundAnchor(t *testing.T) {
	useLocal(t)
	oldH, oldF := homeDir, fallbackHome
	t.Cleanup(func() { homeDir, fallbackHome = oldH, oldF })
	for _, n := range []int{39, 40, 41, 42} {
		base := t.TempDir()
		home := filepath.Join(base, "home")
		_ = os.MkdirAll(home, 0o700)
		_ = os.MkdirAll(filepath.Join(base, "w", "a"), 0o700)
		_ = os.MkdirAll(filepath.Join(base, "o"), 0o700)
		prev := filepath.Join(base, "w", "a", "t1")
		for k := n - 1; k >= 1; k-- {
			l := filepath.Join(base, "o", fmt.Sprintf("hop%d", k))
			_ = os.Symlink(prev, l)
			prev = l
		}
		_ = os.Symlink(prev, filepath.Join(home, ".zshrc")) // n hops total
		homeDir = func() string { return home }
		fallbackHome = func() string { return home }
		_, kerr := os.Stat(filepath.Join(home, ".zshrc"))
		c := filepath.Join(base, "w", "a")
		_, err := bindSource(&runOptions{MountRoots: []string{c}}, c)
		t.Logf("hops=%d kernel stat err=%v ; mount w/a err=%v", n, kerr, err)
		// the anchor is a chain of n links ending below w/a: either the walk
		// follows it (w/a is then an ancestor of the target: refused) or it gives
		// up (anchor unresolvable: fail closed, refused). Accepting is a bypass.
		if err == nil {
			t.Errorf("hops=%d: w/a accepted although ~/.zshrc resolves (or fails to) into it", n)
		}
	}
}
