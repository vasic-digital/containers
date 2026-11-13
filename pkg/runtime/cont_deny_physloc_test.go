package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// N1 (round-3 reviewer): HOME is a symlink and every credential entry below
// it is itself a symlink elsewhere. The PHYSICAL directory that holds those
// link entries must not be mountable (a container could replace the links).
func TestCDS_R3_PhysicalLinkLocationsProtected(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	real := filepath.Join(base, "realhome")
	disk := filepath.Join(base, "disk")
	for _, d := range []string{filepath.Join(real, ".local"), filepath.Join(real, ".config"), filepath.Join(real, "work"),
		filepath.Join(disk, "bin"), filepath.Join(disk, "share"), filepath.Join(disk, "sysd"), filepath.Join(disk, "gh")} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	require.NoError(t, os.Symlink(filepath.Join(disk, "bin"), filepath.Join(real, ".local", "bin")))
	require.NoError(t, os.Symlink(filepath.Join(disk, "share"), filepath.Join(real, ".local", "share")))
	require.NoError(t, os.Symlink(filepath.Join(disk, "sysd"), filepath.Join(real, ".config", "systemd")))
	require.NoError(t, os.Symlink(filepath.Join(disk, "gh"), filepath.Join(real, ".config", "gh")))
	hl := filepath.Join(base, "h")
	require.NoError(t, os.Symlink(real, hl))
	fakeHome(t, hl)
	oldF := fallbackHome
	fallbackHome = func() string { return "" }
	t.Cleanup(func() { fallbackHome = oldF })
	for _, c := range []string{filepath.Join(real, ".local"), filepath.Join(hl, ".local"), filepath.Join(real, ".config"), filepath.Join(hl, ".config")} {
		refusedDeny(t, "physical location of credential links: "+c, WithRunMountAllowedRoots(c), bm5(c))
	}
	// the symlink to ~/.local placed in a work dir is the same location
	w := filepath.Join(base, "w")
	require.NoError(t, os.MkdirAll(w, 0o700))
	require.NoError(t, os.Symlink(filepath.Join(real, ".local"), filepath.Join(w, "loc")))
	refusedDeny(t, "link to ~/.local", WithRunMountAllowedRoots(w), bm5(filepath.Join(w, "loc")))
	allowed(t, "unrelated work dir", WithRunMountAllowedRoots(filepath.Join(real, "work")), bm5(filepath.Join(real, "work")))
}

// RV1: `..` after a symlinked directory component is applied to the RESOLVED
// directory, not the lexical one. ~/.ssh -> "sl/../keys" where sl is a link to
// <work>/a/b: the credential lives at <work>/a/keys, so <work>/a must be refused.
func TestCDS_R3_DotDotAfterSymlinkedComponent(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	fakeHome(t, home)
	oldF := fallbackHome
	fallbackHome = func() string { return "" }
	t.Cleanup(func() { fallbackHome = oldF })
	deep := filepath.Join(home, "work", "a", "b")
	require.NoError(t, os.MkdirAll(deep, 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "o"), 0o700))
	sl := filepath.Join(base, "o", "sl")
	require.NoError(t, os.Symlink(deep, sl))
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".ssh")))
	// lexical dir of sl is base/o; the physical parent of deep is home/work/a
	require.NoError(t, os.Symlink(sl+"/../keys", filepath.Join(home, ".ssh")))
	a := filepath.Join(home, "work", "a")
	refusedDeny(t, "dotdot through symlinked dir: physical parent of the link target", WithRunMountAllowedRoots(a), bm5(a))
	// the credential itself lives at <work>/a/keys, not <work>/a/b/keys: a
	// mutant that ignores `..` anchors the wrong place and lets this through
	keys := filepath.Join(a, "keys")
	require.NoError(t, os.MkdirAll(keys, 0o700))
	refusedDeny(t, "dotdot: the real credential dir", WithRunMountAllowedRoots(keys), bm5(keys))
}

// N03: remote mode is lexical (no anchor locations), so the ancestor-of-anchor
// rule is the only thing refusing a mount of a parent of a protected subtree.
func TestCDS_R3_RemoteAncestorOfSystemSubtree(t *testing.T) {
	for _, p := range []string{"/", "/etc", "/usr"} {
		d, why := denyHostPathMode(p, "", false)
		require.True(t, d, p)
		require.NotEmpty(t, why, p)
	}
	d, _ := denyHostPathMode("/srv2/a", "", false)
	require.False(t, d)
}
