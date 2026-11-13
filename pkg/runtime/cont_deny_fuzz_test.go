package runtime

// Reviewer (independent) property test for CONT-DENY-STRUCT. Oracle written
// from scratch: an own symlink resolver (component walk with Lstat/Readlink,
// following DANGLING links to their lexical target, loop => "unresolvable"),
// then: a candidate is UNSAFE iff its real path equals a protected anchor's
// real target, is an ancestor of it, or lies under a credential (tree) anchor.

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cdsResolve(p string) (string, bool) {
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
		fi, err := os.Lstat(nx)
		if err != nil { // missing: rest is lexical
			cur = nx
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			hops++
			if hops > 40 {
				return "", false
			}
			t, err := os.Readlink(nx)
			if err != nil {
				return "", false
			}
			tp := strings.Split(strings.TrimPrefix(t, "/"), "/")
			if filepath.IsAbs(t) {
				cur = "/"
			}
			parts = append(tp, parts...)
			continue
		}
		cur = nx
	}
	return cur, true
}

func cdsUnder(p, r string) bool {
	return p != r && strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/")
}

var cdsCred = []string{".ssh", ".config/gh", ".local/bin", ".aws", ".docker"}

func cdsOracleUnsafe(cand, home string) (bool, string) {
	rc, ok := cdsResolve(cand)
	if !ok {
		return false, "unresolvable candidate"
	}
	type an struct {
		p    string
		tree bool
	}
	as := []an{{home, false}}
	for _, c := range cdsCred {
		as = append(as, an{filepath.Join(home, c), true})
	}
	for _, a := range as {
		ra, ok := cdsResolve(a.p)
		if !ok {
			continue
		}
		if rc == ra || cdsUnder(ra, rc) || (a.tree && cdsUnder(rc, ra)) {
			return true, fmt.Sprintf("anchor %s -> %s vs cand %s -> %s", a.p, ra, cand, rc)
		}
	}
	return false, rc
}

func TestCDS_R2_SymlinkFuzz(t *testing.T) {
	useLocal(t)
	seed := int64(20261113)
	rng := rand.New(rand.NewSource(seed))
	bypass, fp, agree := 0, 0, 0
	bypassDang := 0
	var bypassOther []string
	var bypassEx, fpEx []string
	for iter := 0; iter < 400; iter++ {
		base := t.TempDir()
		mk := func(p string) { _ = os.MkdirAll(p, 0o700) }
		realHome := filepath.Join(base, "realhome")
		mk(realHome)
		home := realHome
		if rng.Intn(2) == 0 {
			home = filepath.Join(base, "h")
			_ = os.Symlink(realHome, home)
		}
		mk(filepath.Join(realHome, "work", "proj"))
		mk(filepath.Join(base, "out", "a"))
		for _, c := range cdsCred {
			p := filepath.Join(realHome, c)
			mk(filepath.Dir(p))
			switch rng.Intn(8) {
			case 0, 1:
				mk(p)
			case 2:
				mk(filepath.Join(base, "out", "a", "c"+filepath.Base(c)))
				_ = os.Symlink(filepath.Join(base, "out", "a", "c"+filepath.Base(c)), p)
			case 3:
				mk(filepath.Join(realHome, "work", "proj", "c"+filepath.Base(c)))
				_ = os.Symlink(filepath.Join(home, "work", "proj", "c"+filepath.Base(c)), p)
			case 4: // dangling into a work dir
				_ = os.Symlink(filepath.Join(home, "work", "proj", "dang"+filepath.Base(c)), p)
			case 5: // dangling outside
				_ = os.Symlink(filepath.Join(base, "out", "a", "dang"+filepath.Base(c)), p)
			case 6: // relative chain
				mk(filepath.Join(base, "out", "rel"+filepath.Base(c)))
				_ = os.Symlink(filepath.Join(base, "out", "lnk"+filepath.Base(c)), p)
				_ = os.Symlink("rel"+filepath.Base(c), filepath.Join(base, "out", "lnk"+filepath.Base(c)))
			case 7: // loop
				_ = os.Symlink(p+"x", p)
				_ = os.Symlink(p, p+"x")
			}
		}
		_ = os.Symlink(filepath.Join(home, ".ssh"), filepath.Join(base, "out", "tossh"))
		_ = os.Symlink(home, filepath.Join(base, "out", "tohome"))
		cands := []string{
			home, realHome, filepath.Join(home, "work"), filepath.Join(home, "work", "proj"),
			filepath.Join(realHome, "work", "proj"), filepath.Join(base, "out"), filepath.Join(base, "out", "a"),
			filepath.Join(home, ".config"), filepath.Join(home, ".local"), filepath.Join(base, "out", "tossh"),
			filepath.Join(base, "out", "tohome"), filepath.Join(base, "out", "tohome", "work"), base,
		}
		for _, c := range cdsCred {
			cands = append(cands, filepath.Join(home, c), filepath.Join(home, c, "sub"))
		}
		cdsFakeHome(home)
		for _, c := range cands {
			if _, err := os.Stat(c); err != nil {
				continue
			}
			o := &runOptions{MountRoots: []string{c}}
			_, err := bindSource(o, c)
			unsafe, why := cdsOracleUnsafe(c, home)
			switch {
			case unsafe && err == nil:
				bypass++
				if strings.Contains(why, "dang") {
					bypassDang++
				} else {
					bypassOther = append(bypassOther, why)
				}
				if len(bypassEx) < 12 {
					bypassEx = append(bypassEx, why)
				}
			case !unsafe && err != nil:
				fp++
				if len(fpEx) < 100000 {
					fpEx = append(fpEx, c+" -> "+why+" : "+err.Error())
				}
			default:
				agree++
			}
		}
	}
	homeDir = cdsSavedHome
	t.Logf("seed=%d agree=%d bypass=%d (dangling=%d other=%d) falsepositive=%d", seed, agree, bypass, bypassDang, len(bypassOther), fp)
	for i, e := range bypassOther {
		if i < 5 {
			t.Logf("OTHER %s", e)
		}
	}
	fpCls := map[string]int{}
	_ = bypassEx
	for _, e := range fpEx {
		fpCls[filepath.Base(strings.Fields(e)[0])]++
	}
	t.Logf("FP sample basenames %v", fpCls)
	if bypass > 0 {
		t.Errorf("%d bypasses", bypass)
	}
}

var cdsSavedHome = homeDir

func cdsFakeHome(h string) { homeDir = func() string { return h } }

// Wrong (but set) HOME: real credential dirs of the passwd home.
func TestCDS_R2_WrongHomeKeepsPasswdDenies(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	real := filepath.Join(base, "realhome")
	_ = os.MkdirAll(filepath.Join(real, ".ssh"), 0o700)
	oldH, oldF := homeDir, fallbackHome
	defer func() { homeDir, fallbackHome = oldH, oldF }()
	fallbackHome = func() string { return real }
	for _, h := range []string{"/", filepath.Join(base, "other"), "relative", real + "/../realhome"} {
		homeDir = func() string { return h }
		o := &runOptions{MountRoots: []string{filepath.Join(real, ".ssh")}}
		_, err := bindSource(o, filepath.Join(real, ".ssh"))
		t.Logf("HOME=%q mount passwd-home/.ssh err=%v", h, err)
		if err == nil {
			t.Errorf("HOME=%q: passwd home ~/.ssh accepted", h)
		}
	}
}
