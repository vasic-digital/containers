package seed

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidLabel(t *testing.T) {
	for _, l := range []string{"a", "smb-lite_1.run", "x0"} {
		if err := ValidLabel(l); err != nil {
			t.Errorf("%q rejected: %v", l, err)
		}
	}
	for _, l := range []string{"", "A", "../x", "a/b", ".hidden", "-x", "a b", "a\x00b", "ü", strings.Repeat("a", 65), "a\\b", "..", "a:b"} {
		if err := ValidLabel(l); !errors.Is(err, ErrInvalidLabel) {
			t.Errorf("%q accepted (%v)", l, err)
		}
	}
}

func TestGenerateEvidence(t *testing.T) {
	_, m := gen(t, sampleSpec())
	b, _ := MarshalManifest(m)
	h := ManifestSHA256(b)
	ev, err := NewGenerateEvidence("lite-smb", m.SpecSHA256, h, m)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Schema != EvidenceSchema || ev.Operation != "generate" || ev.Result != ResultPass ||
		ev.InputsSHA256 != m.SpecSHA256 || ev.ManifestSHA256 != h || ev.Counts.Files == 0 {
		t.Fatalf("%+v", ev)
	}
	if _, err := NewGenerateEvidence("../bad", m.SpecSHA256, h, m); err == nil {
		t.Fatal("bad label accepted")
	}
}

func TestVerifyEvidenceAndWrite(t *testing.T) {
	root, m := gen(t, sampleSpec())
	b, _ := MarshalManifest(m)
	h := ManifestSHA256(b)
	os.Remove(filepath.Join(root, "zeros.bin"))
	r := mustVerify(t, root, m, Options{})
	ev, err := NewVerifyEvidence("lite-smb", h, r)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Result != ResultFail || len(ev.Diffs) != 1 || ev.Counts.ByCode[CodeMissing] != 1 || ev.Operation != "verify" {
		t.Fatalf("%+v", ev)
	}
	dir := t.TempDir()
	p, err := WriteEvidence(dir, ev)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p) != dir || filepath.Base(p) != "lite-smb.verify.json" {
		t.Fatalf("path %s", p)
	}
	raw, _ := os.ReadFile(p)
	var back Evidence
	if err := json.Unmarshal(raw, &back); err != nil || back.ManifestSHA256 != h {
		t.Fatalf("readback: %v", err)
	}
	p2, _ := WriteEvidence(t.TempDir(), ev)
	raw2, _ := os.ReadFile(p2)
	if !bytes.Equal(raw, raw2) {
		t.Fatal("evidence not deterministic")
	}
	for _, bad := range []string{`"time"`, `"timestamp"`} {
		if bytes.Contains(raw, []byte(bad)) {
			t.Fatal("wall clock in evidence")
		}
	}
	ev.Label = "../escape"
	if _, err := WriteEvidence(dir, ev); err == nil {
		t.Fatal("unsafe label written")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.verify.json")); err == nil {
		t.Fatal("escaped")
	}
}

func TestEvidenceTruncatesDiffs(t *testing.T) {
	r := Report{Result: ResultFail}
	for i := 0; i < maxEvidenceDiffs+5; i++ {
		r.Diffs = append(r.Diffs, Diff{Path: "p", Code: CodeExtra})
	}
	ev, _ := NewVerifyEvidence("x", strings.Repeat("a", 64), r)
	if len(ev.Diffs) != maxEvidenceDiffs || !ev.DiffsTruncated {
		t.Fatalf("%d %v", len(ev.Diffs), ev.DiffsTruncated)
	}
}

func TestEvidenceRejectsBadManifestHash(t *testing.T) {
	if _, err := NewVerifyEvidence("x", "short", Report{Result: ResultPass}); err == nil {
		t.Fatal("bad hash accepted")
	}
}
