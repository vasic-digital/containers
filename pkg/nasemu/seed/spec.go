package seed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func badSpec(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidSpec}, args...)...)
}

// ParseSpec decodes a spec strictly: unknown fields and trailing data are errors.
func ParseSpec(data []byte) (Spec, error) {
	var s Spec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Spec{}, badSpec("decode: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Spec{}, badSpec("trailing data after spec")
	}
	return s, nil
}

// SpecHash is the sha256 (hex) of the canonical JSON encoding of the spec.
func SpecHash(s Spec) (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func validText(s string, what string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%s is not valid UTF-8", what)
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return fmt.Errorf("%s contains a control character or backslash", what)
		}
	}
	return nil
}

// ValidatePath checks a tree-relative slash path: no traversal, no absolute
// form, no empty or dot components, bounded lengths.
func ValidatePath(p string) error {
	if p == "" {
		return fmt.Errorf("path is empty")
	}
	if len(p) > maxPathLen {
		return fmt.Errorf("path longer than %d bytes", maxPathLen)
	}
	if err := validText(p, "path"); err != nil {
		return err
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path %q is absolute", p)
	}
	for _, c := range strings.Split(p, "/") {
		switch {
		case c == "":
			return fmt.Errorf("path %q has an empty component", p)
		case c == "." || c == "..":
			return fmt.Errorf("path %q has a %q component", p, c)
		case len(c) > maxComponentLen:
			return fmt.Errorf("path component longer than %d bytes", maxComponentLen)
		}
	}
	return nil
}

func parseMode(s string, def fs.FileMode) (fs.FileMode, error) {
	if s == "" {
		return def, nil
	}
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil || v > 0o777 {
		return 0, fmt.Errorf("mode %q must be octal and at most 0777", s)
	}
	return fs.FileMode(v), nil
}

func validPattern(p Pattern) bool {
	switch p {
	case PatternZero, PatternPRNG, PatternText, PatternWAV, PatternMP3, PatternMP4, PatternRepeat:
		return true
	}
	return false
}

func expand(s Spec) ([]Entry, error) {
	var out []Entry
	for _, e := range s.Entries {
		has := strings.Contains(e.Path, "{n}")
		switch {
		case e.Count < 0 || e.Count > MaxEntries:
			return nil, badSpec("count %d out of range for %q", e.Count, e.Path)
		case e.Count > 0 && !has:
			return nil, badSpec("count set but path %q has no {n}", e.Path)
		case e.Count == 0 && has:
			return nil, badSpec("path %q has {n} but count is 0", e.Path)
		}
		if e.Count == 0 {
			out = append(out, e)
		} else {
			for i := 0; i < e.Count; i++ {
				c := e
				c.Count = 0
				c.Path = strings.ReplaceAll(e.Path, "{n}", strconv.Itoa(i))
				out = append(out, c)
			}
		}
		if len(out) > MaxEntries {
			return nil, badSpec("more than %d entries", MaxEntries)
		}
	}
	return out, nil
}

// Normalize validates a spec and returns the expanded entries, including
// implicit parent directories, sorted so that parents precede children.
func Normalize(s Spec) ([]Resolved, error) {
	if s.Version != SpecVersion {
		return nil, badSpec("version %d unsupported", s.Version)
	}
	if len(s.Entries) == 0 {
		return nil, badSpec("no entries")
	}
	entries, err := expand(s)
	if err != nil {
		return nil, err
	}
	byPath := map[string]*Resolved{}
	fold := map[string]string{}
	var total int64
	claim := func(p string) error {
		if prev, ok := fold[strings.ToLower(p)]; ok && prev != p {
			return badSpec("paths %q and %q collide case-insensitively", prev, p)
		}
		fold[strings.ToLower(p)] = p
		return nil
	}
	for _, e := range entries {
		if err := ValidatePath(e.Path); err != nil {
			return nil, badSpec("%v", err)
		}
		if _, dup := byPath[e.Path]; dup {
			return nil, badSpec("duplicate path %q", e.Path)
		}
		if err := claim(e.Path); err != nil {
			return nil, err
		}
		r := Resolved{Path: e.Path, Kind: e.Kind}
		def := defaultFileMode
		switch e.Kind {
		case KindDir:
			def = defaultDirMode
			if e.Size != 0 || e.Pattern != "" || e.Data != "" || e.Target != "" {
				return nil, badSpec("dir %q must not carry size, pattern, data or target", e.Path)
			}
		case KindFile:
			if e.Size < 0 || e.Size > MaxTotalBytes {
				return nil, badSpec("file %q size %d out of range", e.Path, e.Size)
			}
			if e.Target != "" {
				return nil, badSpec("file %q must not carry a target", e.Path)
			}
			if e.Pattern == "" && e.Size != 0 {
				return nil, badSpec("file %q needs a pattern", e.Path)
			}
			if e.Pattern != "" && !validPattern(e.Pattern) {
				return nil, badSpec("file %q has unknown pattern %q", e.Path, e.Pattern)
			}
			if (e.Pattern == PatternRepeat) != (e.Data != "") {
				return nil, badSpec("file %q: data is required for, and only for, the repeat pattern", e.Path)
			}
			r.Size, r.Pattern, r.Data = e.Size, e.Pattern, e.Data
			total += e.Size
			if total > MaxTotalBytes {
				return nil, badSpec("total file bytes exceed %d", MaxTotalBytes)
			}
		case KindSymlink:
			if e.Target == "" || e.Size != 0 || e.Pattern != "" || e.Data != "" || e.Mode != "" {
				return nil, badSpec("symlink %q needs only a target", e.Path)
			}
			r.Target = e.Target
		default:
			return nil, badSpec("entry %q has unknown kind %q", e.Path, e.Kind)
		}
		if e.Kind != KindSymlink {
			m, err := parseMode(e.Mode, def)
			if err != nil {
				return nil, badSpec("%q: %v", e.Path, err)
			}
			r.Mode = m
		}
		byPath[e.Path] = &r
	}
	explicit := make([]string, 0, len(byPath))
	for p := range byPath {
		explicit = append(explicit, p)
	}
	for _, p := range explicit {
		for a := path.Dir(p); a != "."; a = path.Dir(a) {
			if ex, ok := byPath[a]; ok {
				if ex.Kind != KindDir {
					return nil, badSpec("%q is under %q which is not a directory", p, a)
				}
				continue
			}
			if err := claim(a); err != nil {
				return nil, err
			}
			byPath[a] = &Resolved{Path: a, Kind: KindDir, Mode: defaultDirMode}
		}
	}
	for _, r := range byPath {
		if r.Kind != KindSymlink {
			continue
		}
		if err := validText(r.Target, "symlink target"); err != nil || len(r.Target) > maxPathLen {
			return nil, badSpec("symlink %q: bad target", r.Path)
		}
		if strings.HasPrefix(r.Target, "/") {
			return nil, badSpec("symlink %q: absolute target", r.Path)
		}
		res := path.Join(path.Dir(r.Path), r.Target)
		if res == "." || res == ".." || strings.HasPrefix(res, "../") {
			return nil, badSpec("symlink %q escapes the tree", r.Path)
		}
		if _, ok := byPath[res]; !ok {
			return nil, badSpec("symlink %q target %q is not in the spec", r.Path, res)
		}
	}
	out := make([]Resolved, 0, len(byPath))
	for _, r := range byPath {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
