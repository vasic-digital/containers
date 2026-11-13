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
	if len(data) > MaxSpecBytes {
		return Spec{}, badSpec("spec larger than %d bytes", MaxSpecBytes)
	}
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
		if hostileRune(r) {
			return fmt.Errorf("%s contains a C1 control, bidi control, zero-width, line-separator or combining-diacritic character (U+%04X)", what, r)
		}
	}
	return nil
}

// hostileRune reports runes that are refused in names: C1 controls, bidi
// controls (visual spoofing), zero-width characters and BOM (invisible
// collisions), line/paragraph separators, and the combining diacritical marks
// block U+0300..U+036F (the usual way an NFD name differs from its NFC twin;
// names must therefore be composed). Other non-NFC forms (for example Hangul
// jamo or Indic sequences) are NOT detected: UNCONFIRMED, no x/text dependency.
func hostileRune(r rune) bool {
	switch {
	case r >= 0x80 && r <= 0x9f:
		return true
	case r == 0x061c, r == 0x200b, r == 0x200c, r == 0x200d, r == 0x200e, r == 0x200f,
		r == 0x2028, r == 0x2029, r == 0x2060, r == 0xfeff:
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		return true
	case r >= 0x0300 && r <= 0x036f:
		return true
	}
	return false
}

func foldKey(p string) string { return strings.ToLower(strings.ToUpper(p)) }

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
		if has {
			digits := 1
			if e.Count > 0 {
				digits = len(strconv.Itoa(e.Count - 1))
			}
			if n := strings.Count(e.Path, "{n}"); len(e.Path)+n*(digits-3) > maxPathLen {
				return nil, badSpec("expanded path template longer than %d bytes", maxPathLen)
			}
		} else if len(e.Path) > maxPathLen {
			return nil, badSpec("path longer than %d bytes", maxPathLen)
		}
		if len(out)+max(e.Count, 1) > MaxEntries {
			return nil, badSpec("more than %d entries", MaxEntries)
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
		if prev, ok := fold[foldKey(p)]; ok && prev != p {
			return badSpec("paths %q and %q collide case-insensitively", prev, p)
		}
		fold[foldKey(p)] = p
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
		if strings.Contains(r.Target, "//") || strings.HasSuffix(r.Target, "/") {
			return nil, badSpec("symlink %q: empty target component", r.Path)
		}
		_, res := resolveTarget(func(p string) (Kind, string, bool) {
			x, ok := byPath[p]
			if !ok {
				return "", "", false
			}
			return x.Kind, x.Target, true
		}, r.Path, r.Target, false)
		switch res {
		case resEscape:
			return nil, badSpec("symlink %q escapes the tree (resolved through the spec's own symlinks)", r.Path)
		case resMissing:
			return nil, badSpec("symlink %q target %q is not in the spec", r.Path, r.Target)
		case resNotDir:
			return nil, badSpec("symlink %q target %q passes through a non-directory", r.Path, r.Target)
		case resLoop:
			return nil, badSpec("symlink %q target %q loops or is too deep", r.Path, r.Target)
		}
	}
	out := make([]Resolved, 0, len(byPath))
	for _, r := range byPath {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

type linkRes int

const (
	resOK linkRes = iota
	resEscape
	resMissing
	resNotDir
	resLoop
)

const maxLinkHops = 40

// resolveTarget resolves a symlink target the way the kernel does: component
// by component, expanding every symlink met on the way (so ".." applies to the
// RESOLVED directory, not to the lexical prefix). lookup describes the tree
// (spec or manifest). With lenient set, a component unknown to lookup is
// treated as a plain directory (used by the verifier on live link targets).
func resolveTarget(lookup func(string) (Kind, string, bool), from, target string, lenient bool) (string, linkRes) {
	var stack []string
	if d := path.Dir(from); d != "." {
		stack = strings.Split(d, "/")
	}
	queue := strings.Split(target, "/")
	hops := 0
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			if len(stack) == 0 {
				return "", resEscape
			}
			stack = stack[:len(stack)-1]
			continue
		}
		stack = append(stack, c)
		k, tgt, ok := lookup(strings.Join(stack, "/"))
		if !ok {
			if lenient {
				continue
			}
			return "", resMissing
		}
		switch k {
		case KindSymlink:
			hops++
			if hops > maxLinkHops {
				return "", resLoop
			}
			if strings.HasPrefix(tgt, "/") {
				return "", resEscape
			}
			stack = stack[:len(stack)-1]
			queue = append(strings.Split(tgt, "/"), queue...)
		case KindFile:
			if len(queue) > 0 {
				return "", resNotDir
			}
		}
	}
	if len(stack) == 0 {
		return "", resEscape
	}
	return strings.Join(stack, "/"), resOK
}
