package seed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

func badManifest(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidManifest}, args...)...)
}

// MarshalManifest returns the canonical manifest bytes (indented JSON, final newline).
func MarshalManifest(m Manifest) ([]byte, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ManifestSHA256 is the lowercase hex sha256 of the manifest bytes.
func ManifestSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == strings.ToLower(s)
}

// ParseManifest checks b against the expected sha256 first, then decodes and
// validates it. The expected hash is mandatory: an unhashed manifest is not
// a golden reference.
func ParseManifest(b []byte, wantSHA256 string) (Manifest, error) {
	if wantSHA256 == "" {
		return Manifest{}, ErrManifestHashNeed
	}
	if ManifestSHA256(b) != strings.ToLower(wantSHA256) {
		return Manifest{}, ErrManifestHash
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, badManifest("decode: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Manifest{}, badManifest("trailing data")
	}
	if err := validateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func validateManifest(m Manifest) error {
	if m.Schema != ManifestSchema {
		return badManifest("schema %q", m.Schema)
	}
	if !isHex64(m.SpecSHA256) {
		return badManifest("spec_sha256 malformed")
	}
	if len(m.Entries) == 0 {
		return badManifest("no entries")
	}
	kinds := map[string]Kind{}
	prev := ""
	for i, e := range m.Entries {
		if err := ValidatePath(e.Path); err != nil {
			return badManifest("%v", err)
		}
		if i > 0 && e.Path <= prev {
			return badManifest("entries not strictly sorted at %q", e.Path)
		}
		prev = e.Path
		if e.Size < 0 {
			return badManifest("%q negative size", e.Path)
		}
		if _, err := parseMode(e.Mode, 0); err != nil || len(e.Mode) != 4 {
			return badManifest("%q mode %q", e.Path, e.Mode)
		}
		switch e.Kind {
		case KindDir:
			if e.Size != 0 || e.SHA256 != "" || e.Target != "" {
				return badManifest("dir %q carries file fields", e.Path)
			}
		case KindFile:
			if !isHex64(e.SHA256) || e.Target != "" {
				return badManifest("file %q sha256/target malformed", e.Path)
			}
		case KindSymlink:
			if e.Target == "" || e.SHA256 != "" || e.Size != 0 {
				return badManifest("symlink %q malformed", e.Path)
			}
		default:
			return badManifest("%q unknown kind %q", e.Path, e.Kind)
		}
		kinds[e.Path] = e.Kind
	}
	for _, e := range m.Entries {
		if d := path.Dir(e.Path); d != "." && kinds[d] != KindDir {
			return badManifest("%q has no directory entry for its parent", e.Path)
		}
	}
	return nil
}
