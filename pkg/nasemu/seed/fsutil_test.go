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
