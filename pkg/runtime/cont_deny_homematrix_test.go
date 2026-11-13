package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCDS_R3_HomeMatrix(t *testing.T) {
	useLocal(t)
	oldH, oldF := homeDir, fallbackHome
	t.Cleanup(func() { homeDir, fallbackHome = oldH, oldF })
	base := t.TempDir()
	pw := filepath.Join(base, "pw")
	_ = os.MkdirAll(filepath.Join(pw, ".ssh"), 0o700)
	_ = os.MkdirAll(filepath.Join(pw, "work"), 0o700)
	envh := filepath.Join(base, "envh")
	_ = os.MkdirAll(filepath.Join(envh, ".aws"), 0o700)
	_ = os.Symlink(envh, filepath.Join(base, "envlink"))
	try := func(src string) error {
		_, err := bindSource(&runOptions{MountRoots: []string{src}}, src)
		return err
	}
	cases := []struct {
		name, home string
		pwd        string
	}{
		{"unset", "", pw}, {"slash", "/", pw}, {"relative", "rel/x", pw}, {"unclean", pw + "/../pw", pw},
		{"other-users", "/home/someoneelse", pw}, {"nonexistent", filepath.Join(base, "nope"), pw},
		{"symlinked", filepath.Join(base, "envlink"), pw}, {"env-real", envh, pw},
	}
	for _, c := range cases {
		homeDir = func() string { return c.home }
		fallbackHome = func() string { return c.pwd }
		e1 := try(filepath.Join(pw, ".ssh"))
		e2 := try(filepath.Join(pw, "work"))
		t.Logf("%-12s HOME=%q pw/.ssh refused=%v pw/work err=%v", c.name, c.home, e1 != nil, e2)
		if e1 == nil {
			t.Errorf("%s: passwd ~/.ssh accepted", c.name)
		}
		if e2 != nil {
			t.Errorf("%s: legit pw/work refused: %v", c.name, e2)
		}
		if c.name == "symlinked" || c.name == "env-real" {
			if try(filepath.Join(envh, ".aws")) == nil {
				t.Errorf("%s: $HOME/.aws accepted", c.name)
			}
		}
	}
	// passwd lookup failing
	for _, h := range []string{"", "/", "rel", envh} {
		homeDir = func() string { return h }
		fallbackHome = func() string { return "" }
		err := try(filepath.Join(envh, ".aws"))
		err2 := try(filepath.Join(base, "pw", "work"))
		t.Logf("passwd-fail HOME=%q envh/.aws err=%v ; pw/work err=%v", h, err != nil, err2)
		if err == nil {
			t.Errorf("passwd-fail HOME=%q: accepted credential dir", h)
		}
		if h != envh && err2 == nil {
			t.Errorf("passwd-fail HOME=%q: no usable home but mount accepted", h)
		}
	}
}
