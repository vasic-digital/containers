package seed

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenSampleManifestSHA pins the sample corpus (captured from the first green run on linux/amd64, go1.26.8;
// pinned after implementation, so it guards regressions, not the original design).
// across other Go versions/platforms: only linux/amd64 was run.
const goldenSampleManifestSHA = "eadc86659b30c2f1667e0060cfc83bbd5471e157d62ee666d183b22322e879a2"

func TestGenerateDeterministicThreeRuns(t *testing.T) {
	var first []byte
	for i := 0; i < 3; i++ {
		root, m := gen(t, sampleSpec())
		b, err := MarshalManifest(m)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = b
		} else if !bytes.Equal(first, b) {
			t.Fatalf("run %d manifest differs", i)
		}
		if r := mustVerify(t, root, m, Options{CheckMode: true, CheckMtime: true}); r.Result != ResultPass {
			t.Fatalf("run %d: %+v", i, r.Diffs)
		}
	}
	if got := ManifestSHA256(first); got != goldenSampleManifestSHA {
		t.Fatalf("golden manifest sha drifted: %s", got)
	}
}

func TestSeedChangesContent(t *testing.T) {
	_, a := gen(t, sampleSpec())
	s := sampleSpec()
	s.Seed++
	_, b := gen(t, s)
	ba, _ := MarshalManifest(a)
	bb, _ := MarshalManifest(b)
	if ManifestSHA256(ba) == ManifestSHA256(bb) {
		t.Fatal("different seeds must give different manifests")
	}
	// Same seed, different path => different content for the same size.
	byPath := map[string]string{}
	for _, e := range a.Entries {
		if strings.HasPrefix(e.Path, "bulk/") {
			if prev, ok := byPath[e.SHA256]; ok {
				t.Fatalf("bulk files %s and %s share content", prev, e.Path)
			}
			byPath[e.SHA256] = e.Path
		}
	}
	if len(byPath) != 3 {
		t.Fatalf("expected 3 distinct bulk files, got %d", len(byPath))
	}
}

func TestSpecDiffersChangesSpecHash(t *testing.T) {
	_, a := gen(t, sampleSpec())
	s := sampleSpec()
	s.MtimeUnix++
	_, b := gen(t, s)
	if a.SpecSHA256 == b.SpecSHA256 {
		t.Fatal("spec hash must cover mtime")
	}
}

func TestGeneratedShape(t *testing.T) {
	root, m := gen(t, sampleSpec())
	c := map[string]ManifestEntry{}
	for _, e := range m.Entries {
		c[e.Path] = e
	}
	if e := c["docs/empty.dat"]; e.Size != 0 || e.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty file entry: %+v", e)
	}
	if c["link-to-song"].Kind != KindSymlink || c["link-to-song"].Target != "media/song.mp3" {
		t.Fatalf("symlink entry: %+v", c["link-to-song"])
	}
	if len(c["bulk/f0.bin"].SHA256) != 64 || c["bulk/f2.bin"].Size != 128 {
		t.Fatal("template expansion wrong")
	}
	st, err := os.Stat(filepath.Join(root, "media"))
	if err != nil || st.Mode().Perm() != 0o750 {
		t.Fatalf("dir mode: %v %v", st, err)
	}
	st, _ = os.Stat(filepath.Join(root, "deep", strings.Repeat("d/", 12), "leaf.bin"))
	if st == nil || st.Mode().Perm() != 0o600 || st.ModTime().Unix() != 1700000000 {
		t.Fatalf("file mode/mtime: %v", st)
	}
	st, _ = os.Stat(filepath.Join(root, "emptydir"))
	if st.ModTime().Unix() != 1700000000 {
		t.Fatalf("dir mtime %v", st.ModTime())
	}
	if tgt, err := os.Readlink(filepath.Join(root, "link-to-song")); err != nil || tgt != "media/song.mp3" {
		t.Fatalf("readlink %q %v", tgt, err)
	}
}

func TestPatterns(t *testing.T) {
	root, _ := gen(t, sampleSpec())
	rd := func(p string) []byte {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	w := rd("media/clip one.wav")
	if string(w[0:4]) != "RIFF" || string(w[8:12]) != "WAVE" || len(w) != 4096 {
		t.Fatal("wav header")
	}
	if string(rd("media/song.mp3")[0:3]) != "ID3" {
		t.Fatal("mp3 header")
	}
	if string(rd("media/movie.mp4")[4:8]) != "ftyp" {
		t.Fatal("mp4 header")
	}
	if !bytes.Equal(rd("zeros.bin"), make([]byte, 1000)) {
		t.Fatal("zero pattern")
	}
	txt := rd("docs/ünïcödé 日本語.txt")
	if len(txt) != 500 || !strings.HasPrefix(string(txt), "line 000000 ") {
		t.Fatalf("text pattern: %q", txt[:20])
	}
	if string(rd("docs/"+strings.Repeat("l", 200)+".txt")) != "ababababab" {
		t.Fatal("repeat pattern")
	}
	if bytes.Equal(rd("bulk/f0.bin"), rd("bulk/f1.bin")) {
		t.Fatal("prng files identical")
	}
}

func TestLargeFileSpansBlocksExactSize(t *testing.T) {
	s := Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: []Entry{{Path: "big", Kind: KindFile, Size: 5*1024*1024 + 7, Pattern: PatternPRNG}}}
	root, m := gen(t, s)
	st, _ := os.Stat(filepath.Join(root, "big"))
	if st.Size() != 5*1024*1024+7 || m.Entries[0].Size != st.Size() {
		t.Fatal("size")
	}
}

func TestGenerateRootGuards(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "x"), []byte("x"))
	if _, err := Generate(sampleSpec(), d); err == nil {
		t.Fatal("non-empty root accepted")
	}
	link := filepath.Join(t.TempDir(), "lnk")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(sampleSpec(), link); err == nil {
		t.Fatal("symlink root accepted")
	}
	empty := t.TempDir()
	if _, err := Generate(sampleSpec(), empty); err != nil {
		t.Fatalf("empty existing dir rejected: %v", err)
	}
}

func TestGenerateNeverWritesOutsideRoot(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "tree")
	s := Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: []Entry{{Path: "../escape", Kind: KindFile, Size: 1, Pattern: PatternZero}}}
	if _, err := Generate(s, root); err == nil {
		t.Fatal("traversal accepted")
	}
	if _, err := os.Lstat(filepath.Join(outer, "escape")); err == nil {
		t.Fatal("file escaped root")
	}
}

func TestSpecInvalid(t *testing.T) {
	f := func(e ...Entry) Spec { return Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: e} }
	file := func(p string) Entry { return Entry{Path: p, Kind: KindFile, Size: 1, Pattern: PatternZero} }
	cases := map[string]Spec{
		"dotdot":        f(file("../x")),
		"dotdot-mid":    f(file("a/../../x")),
		"absolute":      f(file("/etc/x")),
		"empty-path":    f(file("")),
		"dot-comp":      f(file("a/./b")),
		"empty-comp":    f(file("a//b")),
		"trailing-sep":  f(file("a/")),
		"backslash":     f(file("a\\b")),
		"control-char":  f(file("a\x01b")),
		"nul":           f(file("a\x00b")),
		"invalid-utf8":  f(file("a\xffb")),
		"long-comp":     f(file(strings.Repeat("x", 256))),
		"duplicate":     f(file("a"), file("a")),
		"case-collide":  f(file("Readme"), file("README")),
		"file-as-dir":   f(file("a"), file("a/b")),
		"neg-size":      f(Entry{Path: "a", Kind: KindFile, Size: -1, Pattern: PatternZero}),
		"no-pattern":    f(Entry{Path: "a", Kind: KindFile, Size: 5}),
		"bad-pattern":   f(Entry{Path: "a", Kind: KindFile, Size: 5, Pattern: "rand"}),
		"repeat-nodata": f(Entry{Path: "a", Kind: KindFile, Size: 5, Pattern: PatternRepeat}),
		"bad-kind":      f(Entry{Path: "a", Kind: "fifo"}),
		"bad-mode":      f(Entry{Path: "a", Kind: KindFile, Pattern: PatternZero, Mode: "0999"}),
		"mode-big":      f(Entry{Path: "a", Kind: KindFile, Pattern: PatternZero, Mode: "04755"}),
		"sym-escape":    f(Entry{Path: "l", Kind: KindSymlink, Target: "../../etc/passwd"}),
		"sym-abs":       f(Entry{Path: "l", Kind: KindSymlink, Target: "/etc/passwd"}),
		"sym-dangling":  f(Entry{Path: "l", Kind: KindSymlink, Target: "nowhere"}),
		"sym-empty":     f(Entry{Path: "l", Kind: KindSymlink}),
		"count-no-n":    f(Entry{Path: "a", Kind: KindFile, Pattern: PatternZero, Count: 2}),
		"n-no-count":    f(Entry{Path: "a{n}", Kind: KindFile, Pattern: PatternZero}),
		"too-big":       f(Entry{Path: "a", Kind: KindFile, Size: MaxTotalBytes + 1, Pattern: PatternZero}),
		"no-entries":    f(),
		"bad-version":   {Version: 2, Entries: []Entry{file("a")}},
		"dir-with-size": f(Entry{Path: "d", Kind: KindDir, Size: 3}),
	}
	for name, s := range cases {
		if _, err := Normalize(s); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), "invalid spec") {
			t.Errorf("%s: wrong error %v", name, err)
		}
	}
}

func TestParseSpecStrict(t *testing.T) {
	b, _ := json.Marshal(sampleSpec())
	if _, err := ParseSpec(b); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(b), `"seed"`, `"sneed"`, 1)
	if _, err := ParseSpec([]byte(bad)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := ParseSpec(append(b, []byte(`{}`)...)); err == nil {
		t.Fatal("trailing data accepted")
	}
	if _, err := ParseSpec([]byte("not json")); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestImplicitParentsInManifest(t *testing.T) {
	_, m := gen(t, Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: []Entry{{Path: "a/b/c.txt", Kind: KindFile, Size: 1, Pattern: PatternZero}}})
	var paths []string
	for _, e := range m.Entries {
		paths = append(paths, e.Path)
	}
	if strings.Join(paths, ",") != "a,a/b,a/b/c.txt" {
		t.Fatalf("got %v", paths)
	}
}
