package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var labelRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

// ValidLabel accepts only labels that are safe as a file-name component.
func ValidLabel(l string) error {
	if len(l) > labelMaxLen || !labelRE.MatchString(l) || strings.Contains(l, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidLabel, l)
	}
	return nil
}

func manifestCounts(m Manifest) Counts {
	var c Counts
	for _, e := range m.Entries {
		switch e.Kind {
		case KindDir:
			c.Dirs++
		case KindFile:
			c.Files++
			c.Bytes += e.Size
		case KindSymlink:
			c.Symlinks++
		}
	}
	return c
}

// NewGenerateEvidence records that a manifest was produced from a spec. It is
// a self-report of the generator, not a verification of any tree.
func NewGenerateEvidence(label, specHash, manifestHash string, m Manifest) (Evidence, error) {
	if err := ValidLabel(label); err != nil {
		return Evidence{}, err
	}
	if !isHex64(specHash) || !isHex64(manifestHash) {
		return Evidence{}, fmt.Errorf("%w: hashes must be 64 lowercase hex", ErrInvalidManifest)
	}
	return Evidence{Schema: EvidenceSchema, Label: label, Operation: "generate",
		InputsSHA256: specHash, ManifestSHA256: manifestHash, Result: ResultPass,
		Counts: manifestCounts(m), Class: "generator-self-report"}, nil
}

// NewVerifyEvidence records the outcome of verifying a live tree.
func NewVerifyEvidence(label, manifestHash string, r Report) (Evidence, error) {
	if err := ValidLabel(label); err != nil {
		return Evidence{}, err
	}
	if !isHex64(manifestHash) {
		return Evidence{}, fmt.Errorf("%w: manifest hash must be 64 lowercase hex", ErrInvalidManifest)
	}
	ev := Evidence{Schema: EvidenceSchema, Label: label, Operation: "verify",
		ManifestSHA256: manifestHash, Result: r.Result, Counts: r.Counts, Class: "tree-verification"}
	ev.Diffs = r.Diffs
	if len(ev.Diffs) > maxEvidenceDiffs {
		ev.Diffs, ev.DiffsTruncated = ev.Diffs[:maxEvidenceDiffs], true
	}
	return ev, nil
}

// WriteEvidence writes <label>.<operation>.json inside dir (which must exist)
// atomically and returns its path.
func WriteEvidence(dir string, e Evidence) (string, error) {
	if err := ValidLabel(e.Label); err != nil {
		return "", err
	}
	if e.Operation != "generate" && e.Operation != "verify" {
		return "", fmt.Errorf("seed: unknown operation %q", e.Operation)
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	rt, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer rt.Close()
	name := e.Label + "." + e.Operation + ".json"
	tmp := "." + name + ".tmp"
	if err := rt.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	if err := rt.Rename(tmp, name); err != nil {
		_ = rt.Remove(tmp)
		return "", err
	}
	return dir + string(os.PathSeparator) + name, nil
}
