//go:build !integration

package runtime

import "os"

func osSymlink(oldname, newname string) error { return os.Symlink(oldname, newname) }
