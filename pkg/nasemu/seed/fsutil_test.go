package seed

import (
	"errors"
	"io/fs"
)

// errFS fails to open one named path.
type errFS struct {
	fs.FS
	bad string
}

func (e errFS) Open(n string) (fs.File, error) {
	if n == e.bad {
		return nil, errors.New("injected read failure")
	}
	return e.FS.Open(n)
}

func (e errFS) ReadLink(n string) (string, error)   { return fs.ReadLink(e.FS, n) }
func (e errFS) Lstat(n string) (fs.FileInfo, error) { return fs.Lstat(e.FS, n) }

// noLinkFS hides every optional interface of the wrapped FS (no ReadLinkFS).
type noLinkFS struct{ inner fs.FS }

func (n noLinkFS) Open(name string) (fs.File, error) { return n.inner.Open(name) }
