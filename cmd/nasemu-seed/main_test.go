package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const specJSON = `{"version":1,"seed":7,"mtime_unix":1700000000,"entries":[
{"path":"a/b.bin","kind":"file","size":300,"pattern":"prng"},
{"path":"empty","kind":"file"},{"path":"l","kind":"symlink","target":"a/b.bin"}]}`

func runCLI(args ...string) (int, string, string) {
	var o, e bytes.Buffer
	c := run(args, &o, &e)
	return c, o.String(), e.String()
}

func setup(t *testing.T) (tree, mpath, hash, evd string) {
	t.Helper()
	d := t.TempDir()
	spec := filepath.Join(d, "spec.json")
	os.WriteFile(spec, []byte(specJSON), 0o644)
	tree, mpath, evd = filepath.Join(d, "tree"), filepath.Join(d, "m.json"), filepath.Join(d, "ev")
	os.Mkdir(evd, 0o755)
	c, out, errs := runCLI("generate", "-spec", spec, "-out", tree, "-manifest-out", mpath, "-evidence-dir", evd, "-label", "cli-test")
	if c != 0 {
		t.Fatalf("generate exit %d: %s", c, errs)
	}
	hash = strings.TrimPrefix(strings.TrimSpace(out), "manifest_sha256=")
	return
}

func TestCLIGenerateVerifyRoundTrip(t *testing.T) {
	tree, mpath, hash, evd := setup(t)
	c, out, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", tree, "-mode", "-mtime", "-evidence-dir", evd, "-label", "cli-test")
	if c != 0 || !strings.HasPrefix(out, "result=PASS") {
		t.Fatalf("exit %d out %s", c, out)
	}
	for _, n := range []string{"cli-test.generate.json", "cli-test.verify.json"} {
		raw, err := os.ReadFile(filepath.Join(evd, n))
		if err != nil || !json.Valid(raw) {
			t.Fatalf("evidence %s: %v", n, err)
		}
	}
}

func TestCLITamperAndGuards(t *testing.T) {
	tree, mpath, hash, _ := setup(t)
	os.WriteFile(filepath.Join(tree, "a", "b.bin"), []byte("evil"), 0o644)
	if c, out, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", tree); c != 1 || !strings.Contains(out, "size_mismatch") {
		t.Fatalf("tamper: exit %d %s", c, out)
	}
	if c, _, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", strings.Repeat("0", 64), "-root", tree); c != 2 {
		t.Fatalf("wrong manifest hash exit %d", c)
	}
	if c, _, _ := runCLI("verify", "-manifest", mpath, "-root", tree); c != 64 {
		t.Fatalf("missing hash exit %d", c)
	}
	if c, _, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", tree, "extra"); c != 64 {
		t.Fatalf("extra positional exit %d", c)
	}
	if c, _, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", tree, "-label", "l"); c != 64 {
		t.Fatalf("label without evidence-dir exit %d", c)
	}
	if c, _, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", tree, "-evidence-dir", t.TempDir()); c != 64 {
		t.Fatalf("evidence-dir without label exit %d", c)
	}
	if c, _, _ := runCLI("bogus"); c != 64 {
		t.Fatalf("usage exit %d", c)
	}
	if c, _, _ := runCLI("generate", "-spec", "x", "-out", "y", "-manifest-out", "z", "-label", "l"); c != 64 {
		t.Fatalf("label without dir exit %d", c)
	}
	if c, _, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", filepath.Join(t.TempDir(), "absent")); c != 2 {
		t.Fatalf("absent root exit %d", c)
	}
}

func TestCLIRefusesManifestOverwriteAndNonEmptyRoot(t *testing.T) {
	d := t.TempDir()
	spec := filepath.Join(d, "spec.json")
	os.WriteFile(spec, []byte(specJSON), 0o644)
	m := filepath.Join(d, "m.json")
	os.WriteFile(m, []byte("keep"), 0o644)
	if c, _, _ := runCLI("generate", "-spec", spec, "-out", filepath.Join(d, "t"), "-manifest-out", m); c != 2 {
		t.Fatalf("overwrite exit %d", c)
	}
	if b, _ := os.ReadFile(m); string(b) != "keep" {
		t.Fatal("manifest overwritten")
	}
	os.Mkdir(filepath.Join(d, "full"), 0o755)
	os.WriteFile(filepath.Join(d, "full", "x"), []byte("x"), 0o644)
	if c, _, _ := runCLI("generate", "-spec", spec, "-out", filepath.Join(d, "full"), "-manifest-out", filepath.Join(d, "m2.json")); c != 2 {
		t.Fatalf("non-empty root exit %d", c)
	}
}

func TestCLIGenerateRejectsExtraArgsAndChecksManifestFirst(t *testing.T) {
	d := t.TempDir()
	spec := filepath.Join(d, "spec.json")
	os.WriteFile(spec, []byte(specJSON), 0o644)
	if c, _, _ := runCLI("generate", "-spec", spec, "-out", filepath.Join(d, "t"), "-manifest-out", filepath.Join(d, "m.json"), "extra"); c != 64 {
		t.Fatalf("extra positional exit %d", c)
	}
	m := filepath.Join(d, "m.json")
	os.WriteFile(m, []byte("keep"), 0o644)
	tree := filepath.Join(d, "tree")
	if c, _, _ := runCLI("generate", "-spec", spec, "-out", tree, "-manifest-out", m); c != 2 {
		t.Fatalf("exit %d", c)
	}
	if _, err := os.Lstat(tree); err == nil {
		t.Fatal("tree written although the manifest file already existed")
	}
}

func TestCLIEscapesHostilePathsInOutput(t *testing.T) {
	tree, mpath, hash, _ := setup(t)
	evil := filepath.Join(tree, "a", "b.bin")
	os.Remove(evil)
	os.Mkdir(filepath.Join(tree, "line\nbreak\tname"), 0o755)
	c, out, _ := runCLI("verify", "-manifest", mpath, "-manifest-sha256", hash, "-root", tree)
	if c != 1 {
		t.Fatalf("exit %d", c)
	}
	if strings.Contains(out, "line\nbreak") || !strings.Contains(out, `"line\nbreak\tname"`) {
		t.Fatalf("path not escaped: %q", out)
	}
	for _, l := range strings.Split(strings.TrimSpace(out), "\n")[1:] {
		if strings.Count(l, "\t") != 2 {
			t.Fatalf("line injection: %q", l)
		}
	}
}
