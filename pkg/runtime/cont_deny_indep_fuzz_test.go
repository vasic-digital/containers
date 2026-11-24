package runtime

// Independent reviewer oracle (round 2): resolution by GNU `readlink -m`
// (coreutils, physical semantics, follows dangling links), anchors = full
// denyHomeChildren list for BOTH homes, unsafe = candidate real path equals or
// is an ancestor of ANY physical location traversed by an anchor (each link
// location) or of its real target, or lies under a tree anchor's real target.

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var rviCache = map[string][2]string{}

func rviRL(p string) (string, bool) {
	if v, ok := rviCache[p]; ok {
		return v[0], v[1] == "1"
	}
	out, err := exec.Command("readlink", "-m", "--", p).Output()
	r := strings.TrimRight(string(out), "\n")
	ok := err == nil && r != ""
	v := "0"
	if ok {
		v = "1"
	}
	rviCache[p] = [2]string{r, v}
	return r, ok
}

func rviAncEq(a, b string) bool { // a == b or a is ancestor of b
	if a == b {
		return true
	}
	if a == "/" {
		return true
	}
	return strings.HasPrefix(b, a+"/")
}

func rviLocs(a string) []string { // physical location of each component of lexical path a
	var out []string
	parts := strings.Split(strings.TrimPrefix(a, "/"), "/")
	cur := ""
	for _, c := range parts {
		parent := "/" + cur
		cur = strings.TrimPrefix(cur+"/"+c, "/")
		pr, ok := rviRL(parent)
		if !ok {
			continue
		}
		out = append(out, filepath.Join(pr, c))
	}
	return out
}

func rviUnsafe(cand string, homes []string, children []string) (bool, string) {
	rc, ok := rviRL(cand)
	if !ok {
		return true, "candidate unresolvable"
	}
	if rc == "/" || rc == "/home" {
		return true, "root"
	}
	for _, s := range denySubtrees {
		if rviAncEq(s, rc) || rviAncEq(rc, s) {
			return true, "system " + s
		}
	}
	for _, h := range homes {
		anchors := []string{h}
		for _, c := range children {
			anchors = append(anchors, filepath.Join(h, c))
		}
		for i, a := range anchors {
			tree := i > 0
			for _, l := range rviLocs(a) {
				if len(l) >= len(h) || true {
					if rviAncEq(rc, l) && !(i == 0 && false) {
						// home node: only the home location itself and its ancestors matter
						if i == 0 || rc != l || true {
							if i == 0 && !rviAncEq(rc, l) {
								continue
							}
							// a candidate strictly UNDER the home is fine for the home anchor
							return true, fmt.Sprintf("anc-of-loc %s of %s", l, a)
						}
					}
				}
			}
			ra, ok := rviRL(a)
			if !ok {
				continue
			}
			if rviAncEq(rc, ra) {
				return true, fmt.Sprintf("anc-of-target %s of %s", ra, a)
			}
			if tree && rviAncEq(ra, rc) {
				return true, fmt.Sprintf("under-target %s of %s", ra, a)
			}
		}
	}
	return false, rc
}

func TestCDS_R3_IndependentFuzz(t *testing.T) {
	useLocal(t)
	oldH, oldF := homeDir, fallbackHome
	t.Cleanup(func() { homeDir, fallbackHome = oldH, oldF })
	seeds := []int64{7, 1113}
	if s := os.Getenv("RV2_SEED"); s != "" {
		var v int64
		fmt.Sscan(s, &v)
		seeds = []int64{v}
	}
	iters := 15 // full evidence run: CDS_ITERS=150 CDS_SEEDS=7,1113,424242,99001
	if os.Getenv("RV2_ITERS") != "" {
		fmt.Sscan(os.Getenv("RV2_ITERS"), &iters)
	}
	if ss := os.Getenv("CDS_SEEDS"); ss != "" {
		seeds = nil
		for _, f := range strings.Split(ss, ",") {
			var v int64
			fmt.Sscan(f, &v)
			seeds = append(seeds, v)
		}
	}
	if s := os.Getenv("CDS_ITERS"); s != "" {
		fmt.Sscan(s, &iters)
	}
	var fpEx []string
	total, bypass, fp := 0, 0, 0
	bypEx := []string{}
	fpCls := map[string]int{}
	for _, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		for it := 0; it < iters; it++ {
			base := t.TempDir()
			rviCache = map[string][2]string{}
			mk := func(p string) { _ = os.MkdirAll(p, 0o700) }
			slots := []string{"w", "w/a", "w/a/b", "o", "o/x", "d"}
			for _, s := range slots {
				if rng.Intn(4) > 0 || s == "w" {
					mk(filepath.Join(base, s))
				}
			}
			mkHome := func(name string) (lex string) {
				real := filepath.Join(base, name+"real")
				mk(filepath.Join(real, "work", "proj"))
				lex = real
				if rng.Intn(3) == 0 {
					lex = filepath.Join(base, name)
					_ = os.Symlink(real, lex)
				}
				return lex
			}
			h1 := mkHome("h1")
			h2 := h1
			if rng.Intn(2) == 0 {
				h2 = mkHome("h2")
			}
			rslot := func() string {
				s := slots[rng.Intn(len(slots))]
				switch rng.Intn(4) {
				case 0:
					return filepath.Join(h1, "work", "proj", fmt.Sprintf("t%d", rng.Intn(3)))
				case 1:
					return filepath.Join(base, s)
				default:
					return filepath.Join(base, s, fmt.Sprintf("t%d", rng.Intn(3)))
				}
			}
			weird := func(target, linkdir string) string {
				switch rng.Intn(7) {
				case 0: // relative
					if r, err := filepath.Rel(linkdir, target); err == nil {
						return r
					}
				case 1:
					return target + "/"
				case 2:
					return strings.Replace(target, base, base+"/./", 1)
				case 3: // escape above root then back down
					return strings.Repeat("../", 30) + strings.TrimPrefix(target, "/")
				case 4: // via a symlinked dir then '..'
					l := filepath.Join(base, "w", fmt.Sprintf("sl%d", rng.Intn(1000)))
					_ = os.Symlink(filepath.Join(h1, "work", "proj"), l)
					return l + "/../../" + filepath.Base(target)
				}
				return target
			}
			for _, h := range []string{h1, h2} {
				rh, _ := filepath.EvalSymlinks(h)
				for _, c := range denyHomeChildren {
					p := filepath.Join(rh, c)
					mk(filepath.Dir(p))
					switch rng.Intn(10) {
					case 0, 1:
						mk(p)
					case 2:
					case 3, 4: // link (maybe dangling)
						_ = os.Symlink(weird(rslot(), filepath.Dir(p)), p)
					case 5: // chain link->link->(dangling?)
						mid := filepath.Join(base, "o", fmt.Sprintf("m%d", rng.Intn(1000)))
						mk(filepath.Dir(mid))
						_ = os.Symlink(weird(rslot(), filepath.Dir(mid)), mid)
						_ = os.Symlink(weird(mid, filepath.Dir(p)), p)
					case 6: // intermediate component link (dir part)
						if d := filepath.Dir(c); d != "." {
							ip := filepath.Join(rh, d)
							if fi, err := os.Lstat(ip); err == nil && fi.IsDir() {
								if es, _ := os.ReadDir(ip); len(es) == 0 {
									_ = os.Remove(ip)
									_ = os.Symlink(weird(rslot(), filepath.Dir(ip)), ip)
								}
							}
						}
					case 7: // loop 2 or 3
						a, b, cc := p, p+"_b", p+"_c"
						if rng.Intn(2) == 0 {
							_ = os.Symlink(b, a)
							_ = os.Symlink(a, b)
						} else {
							_ = os.Symlink(b, a)
							_ = os.Symlink(cc, b)
							_ = os.Symlink(a, cc)
						}
					case 8: // hop chain 39/40/41
						n := 38 + rng.Intn(4)
						prev := rslot()
						for k := n - 1; k >= 1; k-- {
							l := filepath.Join(base, "o", fmt.Sprintf("hop%d_%d", rng.Intn(1e6), k))
							mk(filepath.Dir(l))
							_ = os.Symlink(prev, l)
							prev = l
						}
						_ = os.Symlink(prev, p)
					case 9:
						_ = os.Symlink(weird(filepath.Join(rh, "work", "proj", "k"+filepath.Base(c)), filepath.Dir(p)), p)
					}
				}
			}
			// symlinks inside the work dir pointing at credential locations
			for k := 0; k < 4; k++ {
				c := denyHomeChildren[rng.Intn(len(denyHomeChildren))]
				tgt := filepath.Join(h1, c)
				if rng.Intn(3) == 0 {
					tgt = filepath.Dir(tgt)
				}
				_ = os.Symlink(weird(tgt, filepath.Join(base, "w")), filepath.Join(base, "w", fmt.Sprintf("to%d", k)))
			}
			_ = os.Symlink(strings.Repeat("../", 40), filepath.Join(base, "w", "toroot"))
			_ = os.Symlink("/proc/self/root"+h1, filepath.Join(base, "w", "procself"))
			homeDir = func() string { return h1 }
			fallbackHome = func() string { return h2 }
			homes := []string{h1}
			if h2 != h1 {
				homes = append(homes, h2)
			}
			var cands []string
			_ = filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
				if err == nil && strings.Count(p, "/") < strings.Count(base, "/")+7 {
					cands = append(cands, p)
				}
				return nil
			})
			for _, c := range cands {
				if _, err := os.Stat(c); err != nil {
					continue
				}
				total++
				_, err := bindSource(&runOptions{MountRoots: []string{c}}, c)
				uns, why := rviUnsafe(c, homes, denyHomeChildren)
				if uns && err == nil {
					bypass++
					if len(bypEx) < 15 {
						bypEx = append(bypEx, fmt.Sprintf("seed=%d it=%d %s :: %s", seed, it, c, why))
					}
				}
				if !uns && err != nil {
					fp++
					msg := err.Error()
					k := "other"
					switch {
					case strings.Contains(msg, "symlink loop"):
						k = "loop/hops"
					case strings.Contains(msg, "credential"):
						k = "credential:" + filepath.Base(strings.TrimSuffix(strings.SplitN(msg, "(", 3)[2], "))"))
					case strings.Contains(msg, "home directory"):
						k = "home"
					}
					fpCls[k]++
					if len(fpEx) < 6 {
						lt, _ := os.Readlink(c)
						fpEx = append(fpEx, fmt.Sprintf("it=%d %s link=%q rc=%s :: %s", it, c, lt, why, msg))
					}
				}
			}
		}
	}
	keys := []string{}
	for k := range fpCls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("FP %-50s %d", k, fpCls[k])
	}
	for _, e := range fpEx {
		t.Logf("FPX %s", e)
	}
	for _, e := range bypEx {
		t.Logf("BYPASS %s", e)
	}
	t.Logf("seeds=%v total=%d bypass=%d fp=%d", seeds, total, bypass, fp)
	if bypass > 0 {
		t.Errorf("%d bypasses", bypass)
	}
}
