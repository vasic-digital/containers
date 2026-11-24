package seed

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestManifestRoundTripAndHash(t *testing.T) {
	_, m := gen(t, sampleSpec())
	b, err := MarshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	h := ManifestSHA256(b)
	if len(h) != 64 {
		t.Fatalf("hash %q", h)
	}
	got, err := ParseManifest(b, h)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := MarshalManifest(got)
	if !bytes.Equal(b, b2) {
		t.Fatal("round trip changed bytes")
	}
	if ManifestSHA256([]byte("abc")) != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal("sha256 known answer")
	}
}

func TestManifestHashGuards(t *testing.T) {
	_, m := gen(t, sampleSpec())
	b, _ := MarshalManifest(m)
	h := ManifestSHA256(b)
	if _, err := ParseManifest(b, ""); !errors.Is(err, ErrManifestHashNeed) {
		t.Fatalf("empty hash: %v", err)
	}
	if _, err := ParseManifest(b, strings.Repeat("0", 64)); !errors.Is(err, ErrManifestHash) {
		t.Fatalf("wrong hash: %v", err)
	}
	tampered := bytes.Replace(b, []byte(`"size": 128`), []byte(`"size": 129`), 1)
	if bytes.Equal(tampered, b) {
		t.Fatal("tamper no-op")
	}
	if _, err := ParseManifest(tampered, h); !errors.Is(err, ErrManifestHash) {
		t.Fatalf("tampered bytes: %v", err)
	}
}

func rehash(t *testing.T, m Manifest) ([]byte, string) {
	t.Helper()
	b, err := MarshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	return b, ManifestSHA256(b)
}

func TestParseManifestRejectsHostile(t *testing.T) {
	_, base := gen(t, sampleSpec())
	mut := func(f func(m *Manifest)) Manifest {
		m := base
		m.Entries = append([]ManifestEntry(nil), base.Entries...)
		f(&m)
		return m
	}
	cases := map[string]Manifest{
		"traversal":  mut(func(m *Manifest) { m.Entries[len(m.Entries)-1].Path = "../../etc/passwd" }),
		"bad-schema": mut(func(m *Manifest) { m.Schema = "other" }),
		"dup":        mut(func(m *Manifest) { m.Entries = append(m.Entries, m.Entries[0]) }),
		"no-parent":  mut(func(m *Manifest) { m.Entries = m.Entries[1:] }),
		"bad-sha":    mut(func(m *Manifest) { m.Entries[2].SHA256 = "zz" }),
		"bad-kind":   mut(func(m *Manifest) { m.Entries[0].Kind = "fifo" }),
		"bad-mode":   mut(func(m *Manifest) { m.Entries[0].Mode = "rwx" }),
		"unsorted":   mut(func(m *Manifest) { m.Entries[0], m.Entries[1] = m.Entries[1], m.Entries[0] }),
		"neg-size":   mut(func(m *Manifest) { m.Entries[2].Size = -3 }),
	}
	for name, m := range cases {
		b, h := rehash(t, m)
		if _, err := ParseManifest(b, h); err == nil || !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("%s: err=%v", name, err)
		}
	}
}
