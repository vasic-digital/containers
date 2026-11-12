package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

func kindOf(m fs.FileMode) Kind {
	switch {
	case m&fs.ModeSymlink != 0:
		return KindSymlink
	case m.IsDir():
		return KindDir
	case m.IsRegular():
		return KindFile
	}
	return "other"
}

func escapes(from, target string) bool {
	if strings.HasPrefix(target, "/") {
		return true
	}
	r := path.Join(path.Dir(from), target)
	return r == ".." || strings.HasPrefix(r, "../")
}

// VerifyDir verifies a local directory. The directory is opened through
// os.Root, so no read can leave it, and symlinks are never followed.
func VerifyDir(root string, m Manifest, opt Options) (Report, error) {
	st, err := os.Stat(root)
	if err != nil {
		return Report{}, err
	}
	if !st.IsDir() {
		return Report{}, fmt.Errorf("%s is not a directory", root)
	}
	rt, err := os.OpenRoot(root)
	if err != nil {
		return Report{}, err
	}
	defer rt.Close()
	return Verify(rt.FS(), m, opt), nil
}

// Verify compares fsys with the manifest and reports every exact difference.
// fsys may be a local tree or whatever a client sees over a mounted or served
// protocol. Entries are examined with Lstat semantics and never followed.
func Verify(fsys fs.FS, m Manifest, opt Options) Report {
	rep := Report{Diffs: []Diff{}, Counts: Counts{ByCode: map[Code]int{}}}
	add := func(p string, c Code, format string, a ...any) {
		rep.Diffs = append(rep.Diffs, Diff{Path: p, Code: c, Detail: fmt.Sprintf(format, a...)})
		rep.Counts.ByCode[c]++
	}
	entries := append([]ManifestEntry(nil), m.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	exp := map[string]bool{}
	bad := map[string]bool{}
	for _, e := range entries {
		exp[e.Path] = true
		if opt.SkipSymlinks && e.Kind == KindSymlink {
			rep.Counts.SkippedSymlinks++
			continue
		}
		switch e.Kind {
		case KindDir:
			rep.Counts.Dirs++
		case KindFile:
			rep.Counts.Files++
			rep.Counts.Bytes += e.Size
		case KindSymlink:
			rep.Counts.Symlinks++
		}
		parentBad := false
		for a := path.Dir(e.Path); a != "."; a = path.Dir(a) {
			if bad[a] {
				parentBad = true
				break
			}
		}
		if parentBad {
			bad[e.Path] = true
			add(e.Path, CodeParentInvalid, "an ancestor failed verification; not read")
			continue
		}
		checkEntry(fsys, e, opt, add, bad)
	}
	walkErr := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			add(p, CodeReadError, "walk: %v", err)
			return nil
		}
		if p == "." || exp[p] {
			return nil
		}
		add(p, CodeExtra, "present but not in the manifest")
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	_ = walkErr
	sort.Slice(rep.Diffs, func(i, j int) bool {
		if rep.Diffs[i].Path != rep.Diffs[j].Path {
			return rep.Diffs[i].Path < rep.Diffs[j].Path
		}
		return rep.Diffs[i].Code < rep.Diffs[j].Code
	})
	rep.Result = ResultPass
	for _, d := range rep.Diffs {
		if d.Code != CodeReadError {
			rep.Result = ResultFail
			break
		}
		rep.Result = ResultError
	}
	return rep
}

func checkEntry(fsys fs.FS, e ManifestEntry, opt Options, add func(string, Code, string, ...any), bad map[string]bool) {
	info, err := fs.Lstat(fsys, e.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			add(e.Path, CodeMissing, "not present")
		} else {
			add(e.Path, CodeReadError, "lstat: %v", err)
		}
		bad[e.Path] = true
		return
	}
	if got := kindOf(info.Mode()); got != e.Kind {
		add(e.Path, CodeKindMismatch, "want %s, found %s", e.Kind, got)
		bad[e.Path] = true
		return
	}
	if e.Kind == KindSymlink {
		tgt, err := fs.ReadLink(fsys, e.Path)
		switch {
		case err != nil:
			add(e.Path, CodeReadError, "readlink: %v", err)
		case escapes(e.Path, tgt):
			add(e.Path, CodeSymlinkEscape, "target %q leaves the tree", tgt)
		case tgt != e.Target:
			add(e.Path, CodeTargetMismatch, "want %q, found %q", e.Target, tgt)
		}
		return
	}
	if opt.CheckMode && modeString(info.Mode()) != e.Mode {
		add(e.Path, CodeModeMismatch, "want %s, found %s", e.Mode, modeString(info.Mode()))
	}
	if opt.CheckMtime && info.ModTime().Unix() != e.MtimeUnix {
		add(e.Path, CodeMtimeMismatch, "want %d, found %d", e.MtimeUnix, info.ModTime().Unix())
	}
	if e.Kind != KindFile {
		return
	}
	if info.Size() != e.Size {
		add(e.Path, CodeSizeMismatch, "want %d, found %d", e.Size, info.Size())
		return
	}
	f, err := fsys.Open(e.Path)
	if err != nil {
		add(e.Path, CodeReadError, "open: %v", err)
		return
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, e.Size+1))
	switch {
	case err != nil:
		add(e.Path, CodeReadError, "read: %v", err)
	case n != e.Size:
		add(e.Path, CodeSizeMismatch, "want %d, read %d", e.Size, n)
	case hex.EncodeToString(h.Sum(nil)) != e.SHA256:
		add(e.Path, CodeHashMismatch, "content differs")
	}
}
