package seed

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestVerifyPassAndNegativeControl(t *testing.T) {
	root, m := gen(t, sampleSpec())
	r := mustVerify(t, root, m, Options{CheckMode: true, CheckMtime: true})
	if r.Result != ResultPass || len(r.Diffs) != 0 {
		t.Fatalf("untouched tree: %+v", r)
	}
	if r.Counts.Files == 0 || r.Counts.Dirs == 0 || r.Counts.Symlinks != 1 || r.Counts.Bytes == 0 {
		t.Fatalf("counts %+v", r.Counts)
	}
}

func TestVerifyTamper(t *testing.T) {
	type tc struct {
		name   string
		mutate func(t *testing.T, root string)
		path   string
		code   Code
		opt    Options
	}
	cases := []tc{
		{"flip-byte", func(t *testing.T, r string) {
			p := filepath.Join(r, "media", "song.mp3")
			b, _ := os.ReadFile(p)
			b[100] ^= 1
			write(t, p, b)
			os.Chtimes(p, time.Unix(1700000000, 0), time.Unix(1700000000, 0))
		}, "media/song.mp3", CodeHashMismatch, Options{}},
		{"truncate", func(t *testing.T, r string) {
			os.Truncate(filepath.Join(r, "zeros.bin"), 999)
		}, "zeros.bin", CodeSizeMismatch, Options{}},
		{"extend", func(t *testing.T, r string) {
			f, _ := os.OpenFile(filepath.Join(r, "zeros.bin"), os.O_APPEND|os.O_WRONLY, 0)
			f.Write([]byte{0})
			f.Close()
		}, "zeros.bin", CodeSizeMismatch, Options{}},
		{"missing-file", func(t *testing.T, r string) {
			os.Remove(filepath.Join(r, "docs", "empty.dat"))
		}, "docs/empty.dat", CodeMissing, Options{}},
		{"missing-dir", func(t *testing.T, r string) {
			os.Remove(filepath.Join(r, "emptydir"))
		}, "emptydir", CodeMissing, Options{}},
		{"extra-file", func(t *testing.T, r string) {
			write(t, filepath.Join(r, "intruder.txt"), []byte("x"))
		}, "intruder.txt", CodeExtra, Options{}},
		{"extra-dir", func(t *testing.T, r string) {
			os.Mkdir(filepath.Join(r, "media", "newdir"), 0o755)
		}, "media/newdir", CodeExtra, Options{}},
		{"wrong-mtime", func(t *testing.T, r string) {
			os.Chtimes(filepath.Join(r, "zeros.bin"), time.Unix(5, 0), time.Unix(5, 0))
		}, "zeros.bin", CodeMtimeMismatch, Options{CheckMtime: true}},
		{"wrong-mode", func(t *testing.T, r string) {
			os.Chmod(filepath.Join(r, "zeros.bin"), 0o600)
		}, "zeros.bin", CodeModeMismatch, Options{CheckMode: true}},
		{"symlink-retarget", func(t *testing.T, r string) {
			p := filepath.Join(r, "link-to-song")
			os.Remove(p)
			os.Symlink("zeros.bin", p)
		}, "link-to-song", CodeTargetMismatch, Options{}},
		{"symlink-escape", func(t *testing.T, r string) {
			p := filepath.Join(r, "link-to-song")
			os.Remove(p)
			os.Symlink("../../../../etc/passwd", p)
		}, "link-to-song", CodeSymlinkEscape, Options{}},
		{"file-became-dir", func(t *testing.T, r string) {
			p := filepath.Join(r, "zeros.bin")
			os.Remove(p)
			os.Mkdir(p, 0o755)
		}, "zeros.bin", CodeKindMismatch, Options{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, m := gen(t, sampleSpec())
			c.mutate(t, root)
			r := mustVerify(t, root, m, c.opt)
			if r.Result != ResultFail || !hasCode(r, c.path, c.code) {
				t.Fatalf("want %s on %s, got %s %+v", c.code, c.path, r.Result, r.Diffs)
			}
			if r.Counts.ByCode[c.code] == 0 {
				t.Fatal("ByCode not counted")
			}
		})
	}
}

func TestVerifyIgnoresMtimeModeWhenNotRequested(t *testing.T) {
	root, m := gen(t, sampleSpec())
	os.Chtimes(filepath.Join(root, "zeros.bin"), time.Unix(5, 0), time.Unix(5, 0))
	os.Chmod(filepath.Join(root, "zeros.bin"), 0o600)
	if r := mustVerify(t, root, m, Options{}); r.Result != ResultPass {
		t.Fatalf("%+v", r.Diffs)
	}
}

// A symlink that points at a file with identical content must NOT be followed
// and accepted in place of the regular file.
func TestSymlinkNeverFollowedAsFile(t *testing.T) {
	root, m := gen(t, sampleSpec())
	outside := filepath.Join(t.TempDir(), "same")
	orig, _ := os.ReadFile(filepath.Join(root, "zeros.bin"))
	write(t, outside, orig)
	p := filepath.Join(root, "zeros.bin")
	os.Remove(p)
	os.Symlink(outside, p)
	r := mustVerify(t, root, m, Options{})
	if r.Result != ResultFail || !hasCode(r, "zeros.bin", CodeKindMismatch) {
		t.Fatalf("symlink accepted as file: %+v", r)
	}
}

// A parent directory replaced by a symlink to an outside directory holding
// identical files must not verify, and its children must not be read through it.
func TestParentSymlinkEscape(t *testing.T) {
	root, m := gen(t, sampleSpec())
	outer := filepath.Join(t.TempDir(), "copy")
	if _, err := Generate(sampleSpec(), outer); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(root, "media"))
	os.Symlink(filepath.Join(outer, "media"), filepath.Join(root, "media"))
	r := mustVerify(t, root, m, Options{})
	if r.Result != ResultFail || !hasCode(r, "media", CodeKindMismatch) || !hasCode(r, "media/song.mp3", CodeParentInvalid) {
		t.Fatalf("%+v", r.Diffs)
	}
}

func TestVerifyDirErrors(t *testing.T) {
	_, m := gen(t, sampleSpec())
	if _, err := VerifyDir(filepath.Join(t.TempDir(), "absent"), m, Options{}); err == nil {
		t.Fatal("absent root must error")
	}
	f := filepath.Join(t.TempDir(), "f")
	write(t, f, []byte("x"))
	if _, err := VerifyDir(f, m, Options{}); err == nil {
		t.Fatal("file root must error")
	}
}

// Golden-good / golden-bad fixtures for the verifier itself, over a bare
// fs.FS (what a client sees over a served protocol; no symlink support).
func fixtureManifest(t *testing.T) (Manifest, fstest.MapFS) {
	t.Helper()
	s := Spec{Version: 1, Seed: 9, MtimeUnix: 1700000000, Entries: []Entry{
		{Path: "a.bin", Kind: KindFile, Size: 64, Pattern: PatternPRNG},
		{Path: "d/b.txt", Kind: KindFile, Size: 40, Pattern: PatternText},
		{Path: "l", Kind: KindSymlink, Target: "a.bin"},
	}}
	root, m := gen(t, s)
	fsys := fstest.MapFS{"d": {Mode: 0o755 | 1<<31, ModTime: time.Unix(1700000000, 0)},
		"l": {Data: []byte("a.bin"), Mode: fs.ModeSymlink | 0o777}}
	for _, e := range m.Entries {
		if e.Kind == KindFile {
			b, _ := os.ReadFile(filepath.Join(root, e.Path))
			fsys[e.Path] = &fstest.MapFile{Data: b, Mode: 0o644, ModTime: time.Unix(1700000000, 0)}
		}
	}
	return m, fsys
}

func TestVerifierGoldenFixtures(t *testing.T) {
	m, good := fixtureManifest(t)
	o := Options{SkipSymlinks: true, CheckMtime: true}
	if r := Verify(good, m, o); r.Result != ResultPass || r.Counts.SkippedSymlinks != 1 {
		t.Fatalf("golden-good failed: %+v", r)
	}
	bad := map[string]func(f fstest.MapFS){
		"a.bin":   func(f fstest.MapFS) { f["a.bin"].Data[3] ^= 1 },
		"d/b.txt": func(f fstest.MapFS) { delete(f, "d/b.txt") },
		"zz.new":  func(f fstest.MapFS) { f["zz.new"] = &fstest.MapFile{Data: []byte("x")} },
	}
	for want, mut := range bad {
		_, f := fixtureManifest(t)
		mut(f)
		r := Verify(f, m, o)
		if r.Result != ResultFail {
			t.Errorf("golden-bad %s passed", want)
		}
		found := false
		for _, d := range r.Diffs {
			if d.Path == want {
				found = true
			}
		}
		if !found {
			t.Errorf("golden-bad %s: diff not reported at path: %+v", want, r.Diffs)
		}
	}
	// Without SkipSymlinks the link is checked too.
	if r := Verify(good, m, Options{CheckMtime: true}); r.Result != ResultPass || r.Counts.Symlinks != 1 {
		t.Fatalf("symlink checked: %+v", r)
	}
	_, noLink := fixtureManifest(t)
	delete(noLink, "l")
	if r := Verify(noLink, m, Options{}); r.Result != ResultFail || !hasCode(r, "l", CodeMissing) {
		t.Fatalf("missing symlink: %+v", r)
	}
}

func TestVerifyReadErrorIsError(t *testing.T) {
	m, good := fixtureManifest(t)
	f := errFS{good, "a.bin"}
	r := Verify(f, m, Options{SkipSymlinks: true})
	if r.Result != ResultError || !hasCode(r, "a.bin", CodeReadError) {
		t.Fatalf("%+v", r)
	}
}

// F1: a ".." after a symlinked directory is applied by the kernel to the
// RESOLVED directory. Lexically t -> x/y/s/../.. is x (inside the spec), but
// really it is the parent of the root.
func escapeSpec() Spec {
	return Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: []Entry{
		{Path: "z", Kind: KindDir},
		{Path: "x/y", Kind: KindDir},
		{Path: "x/y/s", Kind: KindSymlink, Target: "../../z"},
		{Path: "t", Kind: KindSymlink, Target: "x/y/s/../.."},
	}}
}

func TestF1SpecRejectsKernelLevelSymlinkEscape(t *testing.T) {
	if _, err := Normalize(escapeSpec()); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("kernel-level escape accepted by Normalize: %v", err)
	}
	if _, err := Generate(escapeSpec(), filepath.Join(t.TempDir(), "tree")); err == nil {
		t.Fatal("Generate wrote a tree with an escaping symlink")
	}
}

func TestF1VerifierFlagsKernelLevelSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, d := range []string{"z", "x/y"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../../z", filepath.Join(root, "x/y/s")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("x/y/s/../..", filepath.Join(root, "t")); err != nil {
		t.Fatal(err)
	}
	// Real filesystem fact the fix relies on: the kernel lands outside root.
	if got, err := filepath.EvalSymlinks(filepath.Join(root, "t")); err != nil || got != base {
		t.Fatalf("premise: EvalSymlinks(t) = %q, %v; want %q", got, err, base)
	}
	m := Manifest{Schema: ManifestSchema, Entries: []ManifestEntry{
		{Path: "t", Kind: KindSymlink, Mode: "0777", Target: "x/y/s/../.."},
		{Path: "x", Kind: KindDir, Mode: "0755"},
		{Path: "x/y", Kind: KindDir, Mode: "0755"},
		{Path: "x/y/s", Kind: KindSymlink, Mode: "0777", Target: "../../z"},
		{Path: "z", Kind: KindDir, Mode: "0755"},
	}}
	r := mustVerify(t, root, m, Options{})
	if r.Result != ResultFail || !hasCode(r, "t", CodeSymlinkEscape) {
		t.Fatalf("escape not flagged: %+v", r)
	}
	if hasCode(r, "x/y/s", CodeSymlinkEscape) {
		t.Fatalf("benign link flagged: %+v", r.Diffs)
	}
}

// A benign ".." after a symlink that stays in the tree is still accepted.
func TestF1BenignDotDotAfterSymlinkAccepted(t *testing.T) {
	s := Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: []Entry{
		{Path: "a/b", Kind: KindDir},
		{Path: "a/b/f", Kind: KindFile},
		{Path: "a/l", Kind: KindSymlink, Target: "b"},
		{Path: "m", Kind: KindSymlink, Target: "a/l/../b/f"},
	}}
	root, man := gen(t, s)
	if r := mustVerify(t, root, man, Options{}); r.Result != ResultPass {
		t.Fatalf("%+v", r)
	}
}

func TestF1SymlinkLoopRejected(t *testing.T) {
	s := Spec{Version: 1, Seed: 1, MtimeUnix: 1, Entries: []Entry{
		{Path: "a", Kind: KindSymlink, Target: "b"},
		{Path: "b", Kind: KindSymlink, Target: "a"},
	}}
	if _, err := Normalize(s); err == nil {
		t.Fatal("loop accepted")
	}
}

// F3: a bare fs.FS (no ReadLinkFS) must not produce a false PASS for a
// regular file sitting where a symlink belongs.
func TestF3VerifyRefusesFSWithoutReadLink(t *testing.T) {
	m, good := fixtureManifest(t)
	good["l"] = &fstest.MapFile{Data: []byte("a.bin"), Mode: 0o644} // file where a link belongs
	r := Verify(noLinkFS{good}, m, Options{})
	if r.Result != ResultError || !hasCode(r, ".", CodeReadError) {
		t.Fatalf("want ERROR/read_error, got %+v", r)
	}
	r = Verify(noLinkFS{good}, m, Options{SkipSymlinks: true})
	if r.Result != ResultError {
		t.Fatalf("skip-symlinks must not bypass: %+v", r)
	}
	// With ReadLinkFS the same tampering is a kind mismatch.
	if r := Verify(good, m, Options{}); r.Result != ResultFail || !hasCode(r, "l", CodeKindMismatch) {
		t.Fatalf("%+v", r)
	}
}

// F4: SkipSymlinks must not hide a regular file where a symlink belongs.
func TestF4SkipSymlinksStillChecksKind(t *testing.T) {
	root, m := gen(t, sampleSpec())
	p := filepath.Join(root, "link-to-song")
	os.Remove(p)
	write(t, p, []byte("regular"))
	r := mustVerify(t, root, m, Options{SkipSymlinks: true})
	if r.Result != ResultFail || !hasCode(r, "link-to-song", CodeKindMismatch) || r.Counts.SkippedSymlinks != 1 {
		t.Fatalf("%+v", r)
	}
	os.Remove(p)
	if r := mustVerify(t, root, m, Options{SkipSymlinks: true}); !hasCode(r, "link-to-song", CodeMissing) {
		t.Fatalf("missing skipped symlink not reported: %+v", r)
	}
	// A retargeted link is exactly what SkipSymlinks is allowed to ignore.
	os.Symlink("zeros.bin", p)
	if r := mustVerify(t, root, m, Options{SkipSymlinks: true}); r.Result != ResultPass {
		t.Fatalf("%+v", r)
	}
}

// N16: an absolute live target must fail with symlink_escape specifically.
func TestAbsoluteLiveTargetIsEscapeNotMismatch(t *testing.T) {
	root, m := gen(t, sampleSpec())
	p := filepath.Join(root, "link-to-song")
	os.Remove(p)
	os.Symlink("/etc/passwd", p)
	r := mustVerify(t, root, m, Options{})
	if !hasCode(r, "link-to-song", CodeSymlinkEscape) || hasCode(r, "link-to-song", CodeTargetMismatch) {
		t.Fatalf("%+v", r.Diffs)
	}
}
