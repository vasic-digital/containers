package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleSpec exercises every kind and pattern, unicode, spaces, a long name,
// an empty file, deep nesting, a symlink and a templated group.
func sampleSpec() Spec {
	long := strings.Repeat("l", 200) + ".txt"
	deep := "deep/" + strings.Repeat("d/", 12) + "leaf.bin"
	return Spec{
		Version: SpecVersion, Seed: 20261113, MtimeUnix: 1700000000,
		Entries: []Entry{
			{Path: "media", Kind: KindDir, Mode: "0750"},
			{Path: "media/clip one.wav", Kind: KindFile, Size: 4096, Pattern: PatternWAV},
			{Path: "media/song.mp3", Kind: KindFile, Size: 3000, Pattern: PatternMP3},
			{Path: "media/movie.mp4", Kind: KindFile, Size: 70000, Pattern: PatternMP4},
			{Path: "docs/ünïcödé 日本語.txt", Kind: KindFile, Size: 500, Pattern: PatternText},
			{Path: "docs/" + long, Kind: KindFile, Size: 10, Pattern: PatternRepeat, Data: "ab"},
			{Path: "docs/empty.dat", Kind: KindFile},
			{Path: deep, Kind: KindFile, Size: 33, Pattern: PatternPRNG, Mode: "0600"},
			{Path: "zeros.bin", Kind: KindFile, Size: 1000, Pattern: PatternZero},
			{Path: "bulk/f{n}.bin", Kind: KindFile, Count: 3, Size: 128, Pattern: PatternPRNG},
			{Path: "link-to-song", Kind: KindSymlink, Target: "media/song.mp3"},
			{Path: "emptydir", Kind: KindDir},
		},
	}
}

func gen(t *testing.T, s Spec) (string, Manifest) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tree")
	m, err := Generate(s, root)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return root, m
}

func mustVerify(t *testing.T, root string, m Manifest, o Options) Report {
	t.Helper()
	r, err := VerifyDir(root, m, o)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return r
}

func hasCode(r Report, p string, c Code) bool {
	for _, d := range r.Diffs {
		if d.Path == p && d.Code == c {
			return true
		}
	}
	return false
}

func write(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
