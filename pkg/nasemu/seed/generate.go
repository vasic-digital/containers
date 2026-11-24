package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"
)

func modeString(m fs.FileMode) string { return fmt.Sprintf("%04o", uint32(m.Perm())) }

// Generate writes the tree described by spec under root and returns its golden
// manifest. root must be absent or an empty real directory (not a symlink).
// On error a partially written tree may remain; the caller owns root.
func Generate(spec Spec, root string) (Manifest, error) {
	res, err := Normalize(spec)
	if err != nil {
		return Manifest{}, err
	}
	specHash, err := SpecHash(spec)
	if err != nil {
		return Manifest{}, err
	}
	if err := prepareRoot(root); err != nil {
		return Manifest{}, err
	}
	rt, err := os.OpenRoot(root)
	if err != nil {
		return Manifest{}, err
	}
	defer rt.Close()
	mt := time.Unix(spec.MtimeUnix, 0)
	m := Manifest{Schema: ManifestSchema, SpecSHA256: specHash}
	var dirs []Resolved
	for _, r := range res {
		me := ManifestEntry{Path: r.Path, Kind: r.Kind}
		switch r.Kind {
		case KindDir:
			if err := rt.Mkdir(r.Path, 0o700); err != nil {
				return Manifest{}, err
			}
			dirs = append(dirs, r)
			me.Mode, me.MtimeUnix = modeString(r.Mode), spec.MtimeUnix
		case KindFile:
			sum, err := writeFile(rt, spec.Seed, r)
			if err != nil {
				return Manifest{}, err
			}
			if err := rt.Chmod(r.Path, r.Mode); err != nil {
				return Manifest{}, err
			}
			if err := rt.Chtimes(r.Path, mt, mt); err != nil {
				return Manifest{}, err
			}
			me.Size, me.SHA256, me.Mode, me.MtimeUnix = r.Size, sum, modeString(r.Mode), spec.MtimeUnix
		case KindSymlink:
			if err := rt.Symlink(r.Target, r.Path); err != nil {
				return Manifest{}, err
			}
			me.Mode, me.Target = "0777", r.Target
		}
		m.Entries = append(m.Entries, me)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := rt.Chmod(dirs[i].Path, dirs[i].Mode); err != nil {
			return Manifest{}, err
		}
		if err := rt.Chtimes(dirs[i].Path, mt, mt); err != nil {
			return Manifest{}, err
		}
	}
	return m, nil
}

func prepareRoot(root string) error {
	st, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(root, 0o755)
	}
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%w: %s is not a real directory", ErrRootNotEmpty, root)
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(ents) != 0 {
		return fmt.Errorf("%w: %s", ErrRootNotEmpty, root)
	}
	return nil
}

func writeFile(rt *os.Root, seed uint64, r Resolved) (string, error) {
	f, err := rt.OpenFile(r.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), newContentReader(seed, r))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if n != r.Size {
		return "", fmt.Errorf("wrote %d bytes for %q, want %d", n, r.Path, r.Size)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
