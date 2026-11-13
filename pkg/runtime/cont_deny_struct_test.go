package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// CONT-DENY-STRUCT RED probes (reviewer round-5 N1/N2/N3): written against
// the pre-rework deny logic using only seams that already existed.

func mkHome(t *testing.T) (base, home string) {
	t.Helper()
	base = t.TempDir()
	home = filepath.Join(base, "home")
	for _, d := range []string{"work", ".config/gh", ".config/other", ".local/share/keyrings", ".local/bin", ".ssh"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, d), 0o700))
	}
	return
}

// N1: credential child is a symlink to a directory outside the lexical child.
func TestCDS_N1_SymlinkedCredentialChild(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	dot := filepath.Join(home, "dotfiles", "ssh")
	require.NoError(t, os.MkdirAll(dot, 0o700))
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".ssh")))
	require.NoError(t, os.Symlink(dot, filepath.Join(home, ".ssh")))
	_ = base
	fakeHome(t, home)
	refusedDeny(t, "source = real target of ~/.ssh", WithRunMountAllowedRoots(filepath.Join(home, "dotfiles")), bm5(dot))
	refusedDeny(t, "source = ~/.ssh symlink", WithRunMountAllowedRoots(filepath.Join(home, "dotfiles")), bm5(filepath.Join(home, ".ssh")))
	allowed(t, "ctrl work", WithRunMountAllowedRoots(filepath.Join(home, "work")), bm5(filepath.Join(home, "work")))
}

// N2: mounting an ancestor of a credential child.
func TestCDS_N2_AncestorOfCredentialChild(t *testing.T) {
	useLocal(t)
	_, home := mkHome(t)
	fakeHome(t, home)
	for _, a := range []string{".config", ".local", ".local/share"} {
		p := filepath.Join(home, a)
		refusedDeny(t, "ancestor "+a, WithRunMountAllowedRoots(p), bm5(p))
		refusedDeny(t, "ancestor via wide root "+a, WithRunMountAllowedRoots(home), bm5(p))
	}
	allowed(t, "sibling .config/other", WithRunMountAllowedRoots(filepath.Join(home, ".config", "other")), bm5(filepath.Join(home, ".config", "other")))
}

// N3: HOME unset / unresolvable in local mode must fail closed.
func TestCDS_N3_HomeUnsetFailsClosed(t *testing.T) {
	useLocal(t)
	_, home := mkHome(t)
	old := homeDir
	homeDir = func() string { return "" }
	oldF := fallbackHome
	fallbackHome = func() string { return "" }
	t.Cleanup(func() { homeDir = old; fallbackHome = oldF })
	refusedDeny(t, "no home: ~/.ssh", WithRunMountAllowedRoots(filepath.Join(home, ".ssh")), bm5(filepath.Join(home, ".ssh")))
	refusedDeny(t, "no home: ordinary work dir", WithRunMountAllowedRoots(filepath.Join(home, "work")), bm5(filepath.Join(home, "work")))
}

// N3b: $HOME unusable but passwd gives a home: the credential denies still hold.
func TestCDS_N3_HomeFromPasswdFallback(t *testing.T) {
	useLocal(t)
	_, home := mkHome(t)
	old := homeDir
	homeDir = func() string { return "" }
	oldF := fallbackHome
	fallbackHome = func() string { return home }
	t.Cleanup(func() { homeDir = old; fallbackHome = oldF })
	refusedDeny(t, "fallback home: ~/.ssh", WithRunMountAllowedRoots(filepath.Join(home, ".ssh")), bm5(filepath.Join(home, ".ssh")))
	refusedDeny(t, "fallback home: ~/.config ancestor", WithRunMountAllowedRoots(filepath.Join(home, ".config")), bm5(filepath.Join(home, ".config")))
	allowed(t, "fallback home: work", WithRunMountAllowedRoots(filepath.Join(home, "work")), bm5(filepath.Join(home, "work")))
}

// N3c: a relative / unclean home is not trusted either.
func TestCDS_N3_BadHomeFailsClosed(t *testing.T) {
	useLocal(t)
	_, home := mkHome(t)
	oldF := fallbackHome
	fallbackHome = func() string { return "" }
	t.Cleanup(func() { fallbackHome = oldF })
	for _, h := range []string{"relative/home", home + "/../home", "/"} {
		fakeHome(t, h)
		refusedDeny(t, "bad home "+h, WithRunMountAllowedRoots(filepath.Join(home, "work")), bm5(filepath.Join(home, "work")))
	}
}

// Symlinked ~/.config and ~/.local, HOME itself a symlink, intermediate
// symlink, trailing slash and ".." forms.
func TestCDS_SymlinkedAncestorsAndFormsRefused(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	other := filepath.Join(base, "elsewhere")
	require.NoError(t, os.MkdirAll(filepath.Join(other, "cfg", "gh"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(other, "loc", "share", "keyrings"), 0o700))
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".config")))
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".local")))
	require.NoError(t, os.Symlink(filepath.Join(other, "cfg"), filepath.Join(home, ".config")))
	require.NoError(t, os.Symlink(filepath.Join(other, "loc"), filepath.Join(home, ".local")))
	fakeHome(t, home)
	// real targets of the symlinked ancestors and their credential children
	for _, p := range []string{filepath.Join(other, "cfg"), filepath.Join(other, "cfg", "gh"),
		filepath.Join(other, "loc"), filepath.Join(other, "loc", "share"), filepath.Join(other, "loc", "share", "keyrings"),
		filepath.Join(home, ".config"), filepath.Join(home, ".local")} {
		refusedDeny(t, "symlinked ancestor "+p, WithRunMountAllowedRoots(p), bm5(p))
	}
	// trailing slash and ".." are refused as unclean
	refusedDeny(t, "trailing slash", WithRunMountAllowedRoots(home), bm5(filepath.Join(home, ".ssh")+"/"))
	refusedDeny(t, "dotdot after symlink", WithRunMountAllowedRoots(home), bm5(filepath.Join(home, ".config", "..", ".ssh")))
	// HOME itself a symlink; caller names real path or the link
	link := filepath.Join(base, "hl")
	require.NoError(t, os.Symlink(home, link))
	fakeHome(t, link)
	refusedDeny(t, "HOME link: real .ssh", WithRunMountAllowedRoots(filepath.Join(home, ".ssh")), bm5(filepath.Join(home, ".ssh")))
	refusedDeny(t, "HOME link: link .ssh", WithRunMountAllowedRoots(filepath.Join(link, ".ssh")), bm5(filepath.Join(link, ".ssh")))
	refusedDeny(t, "HOME link: real home", WithRunMountAllowedRoots(home), bm5(home))
	allowed(t, "HOME link: work via link", WithRunMountAllowedRoots(filepath.Join(link, "work")), bm5(filepath.Join(link, "work")))
	allowed(t, "HOME link: work via real", WithRunMountAllowedRoots(filepath.Join(home, "work")), bm5(filepath.Join(home, "work")))
}

// N4: system and tmp anchors are injectable; their RESOLVED forms are
// exercised through the real deny path.
func TestCDS_N4_ResolvedSystemAndTmpAnchors(t *testing.T) {
	base := t.TempDir()
	realSys := filepath.Join(base, "realsys")
	realTmp := filepath.Join(base, "realtmp")
	for _, d := range []string{realSys, filepath.Join(realTmp, "podman-run-1"), filepath.Join(realTmp, "ordinary"), filepath.Join(base, "free")} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	sysLink := filepath.Join(base, "syslink")
	tmpLink := filepath.Join(base, "tmplink")
	require.NoError(t, os.Symlink(realSys, sysLink))
	require.NoError(t, os.Symlink(realTmp, tmpLink))
	oldS, oldT := denySubtrees, tmpAnchor
	denySubtrees = []string{sysLink}
	tmpAnchor = tmpLink
	t.Cleanup(func() { denySubtrees, tmpAnchor = oldS, oldT })
	home := filepath.Join(base, "h")
	require.NoError(t, os.MkdirAll(home, 0o755))

	// system anchor: lexical anchor is the LINK, caller names the REAL dir
	for _, p := range []string{realSys, filepath.Join(realSys, "x"), sysLink, filepath.Join(sysLink, "x")} {
		d, _ := denyHostPathMode(p, home, true)
		require.True(t, d, p)
	}
	// remote (lexical): the real dir is not known to be an alias
	d, _ := denyHostPathMode(filepath.Join(realSys, "x"), "", false)
	require.False(t, d)
	d, _ = denyHostPathMode(filepath.Join(sysLink, "x"), "", false)
	require.True(t, d)
	// tmp anchor symlinked: state prefixes denied through the real path too
	for _, p := range []string{filepath.Join(realTmp, "podman-run-1", "s"), filepath.Join(tmpLink, "podman-run-1", "s")} {
		d, _ := denyHostPathMode(p, home, true)
		require.True(t, d, p)
	}
	d, _ = denyHostPathMode(filepath.Join(realTmp, "ordinary", "s"), home, true)
	require.False(t, d)
	d, _ = denyHostPathMode(filepath.Join(base, "free"), home, true)
	require.False(t, d)
	// tmp anchor itself (and via its real path) is not mountable
	for _, p := range []string{tmpLink, realTmp} {
		d, _ := denyHostPathMode(p, home, true)
		require.True(t, d, p)
	}
}

// Resolved-home pass: only the RESOLVED form of the home matches.
func TestCDS_ResolvedHomeFormMatters(t *testing.T) {
	base, home := mkHome(t)
	link := filepath.Join(base, "hl")
	require.NoError(t, os.Symlink(home, link))
	d, _ := denyHostPathMode(filepath.Join(home, ".ssh"), link, true)
	require.True(t, d)
	d, _ = denyHostPathMode(home, link, true)
	require.True(t, d)
	d, _ = denyHostPathMode(filepath.Join(home, "work"), link, true)
	require.False(t, d)
}

// Property-style, class-exhaustive: home real/symlinked x each protected child
// real/symlinked/absent x {anchor, ancestor, descendant, sibling-legit}.
func TestCDS_Property_ClassExhaustive(t *testing.T) {
	children := []string{".ssh", ".config/gh", ".local/share/keyrings"}
	const (
		real_ = iota
		symlinked
		absent
	)
	for homeLinked := 0; homeLinked < 2; homeLinked++ {
		for code := 0; code < 27; code++ {
			states := []int{code % 3, (code / 3) % 3, (code / 9) % 3}
			base := t.TempDir()
			realHome := filepath.Join(base, "realhome")
			require.NoError(t, os.MkdirAll(filepath.Join(realHome, "work"), 0o755))
			require.NoError(t, os.MkdirAll(filepath.Join(realHome, ".config", "other"), 0o755))
			require.NoError(t, os.MkdirAll(filepath.Join(realHome, ".local", "share", "other"), 0o755))
			home := realHome
			if homeLinked == 1 {
				home = filepath.Join(base, "homelink")
				require.NoError(t, os.Symlink(realHome, home))
			}
			var targets []string // real targets of symlinked children
			for i, c := range children {
				p := filepath.Join(realHome, c)
				switch states[i] {
				case real_:
					require.NoError(t, os.MkdirAll(p, 0o700))
				case symlinked:
					tgt := filepath.Join(realHome, "dotfiles", filepath.Base(c)+"-t")
					require.NoError(t, os.MkdirAll(tgt, 0o700))
					require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
					require.NoError(t, os.Symlink(tgt, p))
					targets = append(targets, tgt)
				}
			}
			var deny, allow []string
			for _, h := range []string{home, realHome} {
				deny = append(deny, h, filepath.Dir(h))
				allow = append(allow, filepath.Join(h, "work"), filepath.Join(h, "work", "deep"),
					filepath.Join(h, ".config", "other"), filepath.Join(h, ".local", "share", "other"),
					filepath.Join(h, ".ssh-notes"), filepath.Join(h, ".config", "gh2"), filepath.Join(h, ".local", "share", "keyrings2"))
				for _, c := range children {
					a := filepath.Join(h, c)
					deny = append(deny, a, filepath.Join(a, "sub")) // anchor, descendant
					for d := filepath.Dir(a); d != h && d != "."; d = filepath.Dir(d) {
						deny = append(deny, d) // ancestors of the child within home
					}
				}
			}
			for _, tgt := range targets {
				deny = append(deny, tgt, filepath.Join(tgt, "sub"))
			}
			if len(targets) > 0 {
				for _, h := range []string{home, realHome} {
					deny = append(deny, filepath.Join(h, "dotfiles")) // ancestor of a resolved credential dir
				}
			}
			for _, p := range deny {
				d, _ := denyHostPathMode(p, home, true)
				require.True(t, d, "homeLinked=%d states=%v DENY %s", homeLinked, states, p)
			}
			for _, p := range allow {
				d, _ := denyHostPathMode(p, home, true)
				require.False(t, d, "homeLinked=%d states=%v ALLOW %s", homeLinked, states, p)
			}
		}
	}
}

// Seam: with an empty credential-child list the home node anchor alone must
// still deny the home and its ancestor, and still accept a work dir.
func TestCDS_HomeNodeAnchorStandsAlone(t *testing.T) {
	old := denyHomeChildren
	denyHomeChildren = nil
	t.Cleanup(func() { denyHomeChildren = old })
	_, home := mkHome(t)
	for _, p := range []string{home, filepath.Dir(home)} {
		d, _ := denyHostPathMode(p, home, true)
		require.True(t, d, p)
	}
	d, _ := denyHostPathMode(filepath.Join(home, "work"), home, true)
	require.False(t, d)
}

// A candidate whose LEXICAL form is harmless but whose resolved form lands in
// a tmp runtime-state directory.
func TestCDS_TmpStateReachedOnlyViaResolvedCandidate(t *testing.T) {
	base := t.TempDir()
	realTmp := filepath.Join(base, "rt")
	require.NoError(t, os.MkdirAll(filepath.Join(realTmp, "podman-run-1", "s"), 0o755))
	free := filepath.Join(base, "free")
	require.NoError(t, os.MkdirAll(free, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(realTmp, "podman-run-1"), filepath.Join(free, "v")))
	old := tmpAnchor
	tmpAnchor = realTmp
	t.Cleanup(func() { tmpAnchor = old })
	home := filepath.Join(base, "h")
	require.NoError(t, os.MkdirAll(home, 0o755))
	d, _ := denyHostPathMode(filepath.Join(free, "v", "s"), home, true)
	require.True(t, d)
	d, _ = denyHostPathMode(filepath.Join(free, "v", "s"), "", false) // remote: lexical, no local resolution
	require.False(t, d)
}

// A candidate whose resolved form is another user's home (/home/<x>).
func TestCDS_ResolvedFormIsUserHomeDir(t *testing.T) {
	ents, err := os.ReadDir("/home")
	var name string
	for _, e := range ents {
		if e.IsDir() {
			name = e.Name()
			break
		}
	}
	if err != nil || name == "" {
		t.Skip("no directory below /home on this host") // SKIP-OK: needs a real /home/<user> dir
	}
	base := t.TempDir()
	link := filepath.Join(base, "l")
	require.NoError(t, os.Symlink(filepath.Join("/home", name), link))
	home := "/zz-nohome/h" // unrelated to /home, so no ancestor rule can fire
	d, _ := denyHostPathMode(link, home, true)
	require.True(t, d)
	d, _ = denyHostPathMode(link, "", false) // remote: lexical only
	require.False(t, d)
}

// The source check sees the RESOLVED form (a socket-named file behind a symlink
// in an otherwise acceptable root).
func TestCDS_SourceResolvedFormChecked(t *testing.T) {
	useLocal(t)
	_, home := mkHome(t)
	fakeHome(t, home)
	w := filepath.Join(home, "work")
	require.NoError(t, os.WriteFile(filepath.Join(w, "x.sock"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(w, "x.sock"), filepath.Join(w, "l")))
	refusedDeny(t, "symlink to x.sock", WithRunMountAllowedRoots(w), bm5(filepath.Join(w, "l")))
}

// ---- round 2 (Opus NO-GO F1/F2) ----------------------------------------------

// F1: a credential child that is a DANGLING symlink must still be anchored on
// its (missing) target, also through relative and chained links.
func TestCDS_R2_DanglingCredentialLinks(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	fakeHome(t, home)
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".ssh")))
	proj := filepath.Join(home, "work", "proj")
	require.NoError(t, os.MkdirAll(proj, 0o700))
	require.NoError(t, os.Symlink(filepath.Join(proj, "keys"), filepath.Join(home, ".ssh")))
	refusedDeny(t, "dangling abs target: parent mounted", WithRunMountAllowedRoots(proj), bm5(proj))
	refusedDeny(t, "dangling abs target: dir through it", WithRunMountAllowedRoots(home+"/.ssh"), bm5(home+"/.ssh"))
	allowed(t, "sibling work dir", WithRunMountAllowedRoots(filepath.Join(home, "work", "other")), func() RunOption {
		require.NoError(t, os.MkdirAll(filepath.Join(home, "work", "other"), 0o700))
		return bm5(filepath.Join(home, "work", "other"))
	}())
	// relative dangling target
	require.NoError(t, os.Remove(filepath.Join(home, ".ssh")))
	require.NoError(t, os.Symlink("work/proj/relkeys", filepath.Join(home, ".ssh")))
	refusedDeny(t, "dangling relative target", WithRunMountAllowedRoots(proj), bm5(proj))
	// chain: ~/.ssh -> /out/l1 (existing link) -> dangling under proj
	out := filepath.Join(base, "out")
	require.NoError(t, os.MkdirAll(out, 0o700))
	require.NoError(t, os.Symlink(filepath.Join(proj, "chain"), filepath.Join(out, "l1")))
	require.NoError(t, os.Remove(filepath.Join(home, ".ssh")))
	require.NoError(t, os.Symlink(filepath.Join(out, "l1"), filepath.Join(home, ".ssh")))
	refusedDeny(t, "dangling via chain", WithRunMountAllowedRoots(proj), bm5(proj))
}

// A symlink loop is denied for a candidate (fail closed) and does not hide a
// protected anchor.
func TestCDS_R2_SymlinkLoopFailsClosed(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	a, b := filepath.Join(base, "la"), filepath.Join(base, "lb")
	require.NoError(t, os.Symlink(b, a))
	require.NoError(t, os.Symlink(a, b))
	d, why := denyHostPathMode(filepath.Join(a, "x"), home, true)
	require.True(t, d, why)
	require.Contains(t, why, "symlink loop")
	// a looping credential anchor: lexical form still protected, candidates still judged
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".aws")))
	require.NoError(t, os.Symlink(filepath.Join(home, ".awsx"), filepath.Join(home, ".aws")))
	require.NoError(t, os.Symlink(filepath.Join(home, ".aws"), filepath.Join(home, ".awsx")))
	d, _ = denyHostPathMode(filepath.Join(home, ".aws"), home, true)
	require.True(t, d)
	// N2: an anchor that does not resolve cannot be protected precisely: refuse
	// everything (fail closed), with a reason that names the anchor.
	d, why = denyHostPathMode(filepath.Join(home, "work"), home, true)
	require.True(t, d)
	require.Contains(t, why, "does not resolve")
	require.Contains(t, why, ".aws")
}

// N2: an anchor that is a chain of 41+ links (beyond the walk bound) is not
// silently dropped.
func TestCDS_R3_AnchorHopChainFailsClosed(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	fakeHome(t, home)
	oldF := fallbackHome
	fallbackHome = func() string { return "" }
	t.Cleanup(func() { fallbackHome = oldF })
	work := filepath.Join(home, "work")
	prev := filepath.Join(work, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(base, "o"), 0o700))
	for k := 45; k >= 1; k-- {
		l := filepath.Join(base, "o", fmt.Sprintf("h%d", k))
		require.NoError(t, os.Symlink(prev, l))
		prev = l
	}
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".aws")))
	require.NoError(t, os.Symlink(prev, filepath.Join(home, ".aws")))
	refusedDeny(t, "45-link anchor chain: ordinary work dir", WithRunMountAllowedRoots(work), bm5(work))
	d, why := denyHostPathMode(work, home, true)
	require.True(t, d)
	require.Contains(t, why, "does not resolve")
}

// F2: both $HOME and the passwd home are anchors; "/" or unclean HOME is invalid.
func TestCDS_R2_PasswdHomeAnchorAlongsideHome(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	other := filepath.Join(base, "other")
	require.NoError(t, os.MkdirAll(filepath.Join(other, ".ssh"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(other, "work"), 0o700))
	oldF := fallbackHome
	t.Cleanup(func() { fallbackHome = oldF })
	fallbackHome = func() string { return other }
	for _, h := range []string{home, "/", "relative", other + "/../other"} {
		fakeHome(t, h)
		refusedDeny(t, "passwd ~/.ssh, HOME="+h, WithRunMountAllowedRoots(filepath.Join(other, ".ssh")), bm5(filepath.Join(other, ".ssh")))
		refusedDeny(t, "passwd home itself, HOME="+h, WithRunMountAllowedRoots(other), bm5(other))
		allowed(t, "passwd work dir, HOME="+h, WithRunMountAllowedRoots(filepath.Join(other, "work")), bm5(filepath.Join(other, "work")))
	}
	fakeHome(t, home)
	refusedDeny(t, "HOME ~/.ssh with passwd elsewhere", WithRunMountAllowedRoots(filepath.Join(home, ".ssh")), bm5(filepath.Join(home, ".ssh")))
}

// The error names the escape hatch (mount a subdirectory).
func TestCDS_R2_ErrorHintsSubdirectory(t *testing.T) {
	useLocal(t)
	base, home := mkHome(t)
	fakeHome(t, home)
	_ = base
	o := &runOptions{MountRoots: []string{filepath.Join(home, "work")}}
	_, err := bindSource(o, filepath.Join(home, ".ssh"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "dedicated subdirectory")
}
