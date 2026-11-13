package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCDS_R2_Table(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	_ = os.MkdirAll(filepath.Join(home, "work", "proj"), 0o700)
	_ = os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	cdsFakeHome(home)
	defer func() { homeDir = cdsSavedHome }()
	// dangling credential link into a work dir: the exploit
	_ = os.RemoveAll(filepath.Join(home, ".ssh"))
	_ = os.Symlink(filepath.Join(home, "work", "proj", "keys"), filepath.Join(home, ".ssh"))
	// the exploit: ~/work/proj holds the (missing) target of the dangling ~/.ssh
	{
		w := filepath.Join(home, "work", "proj")
		o := &runOptions{MountRoots: []string{w}}
		if _, err := bindSource(o, w); err == nil {
			t.Errorf("%s accepted although ~/.ssh dangles into it", w)
		}
	}
	for _, c := range []string{
		"/", "/proc/self/root" + home, "/proc/1/root", "/home/otheruser", "/home/otheruser/.ssh",
		home + "/", home + "//work", "work", home + "/work/../.ssh", "/HOME/x", "/tmp", "/tmp/podman-run-1000",
		"/var/home", "/run/user/1000", filepath.Join(home, "work", "proj"),
	} {
		o := &runOptions{MountRoots: []string{c}}
		_, err := bindSource(o, c)
		d, why := denyHostPathMode(filepath.Clean(c), home, true)
		t.Logf("%-50q bind err=%v | deny=%v %s", c, err != nil, d, why)
	}
	o := &runOptions{remote: true, MountRoots: []string{"/srv2"}}
	u, err := bindSource(o, "/srv2/x")
	t.Logf("remote /srv2/x -> %q err=%v", u, err)
	o = &runOptions{remote: true, MountRoots: []string{"/home"}}
	_, err = bindSource(o, "/home/u/.ssh")
	t.Logf("remote /home/u/.ssh err=%v (home anchors not known remotely)", err)
}
