package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCDS_R2_Table: every row ASSERTS the outcome and the refusal class.
func TestCDS_R2_Table(t *testing.T) {
	useLocal(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	_ = os.MkdirAll(filepath.Join(home, "work", "proj"), 0o700)
	cdsFakeHome(home)
	defer func() { homeDir = cdsSavedHome }()
	// dangling credential link into a work dir: the exploit
	_ = os.RemoveAll(filepath.Join(home, ".ssh"))
	_ = os.Symlink(filepath.Join(home, "work", "proj", "keys"), filepath.Join(home, ".ssh"))
	rows := []struct {
		src  string
		want string // substring of the refusal message ("" = the path may not exist: any refusal)
	}{
		{"/", "refused ("},
		{"/proc/self/root" + home, "refused ("},
		{"/proc/1/root", "refused ("},
		{"/home/otheruser", "refused ("},
		{"/home/otheruser/.ssh", ""}, // does not exist locally
		{home + "/", ""},             // not clean
		{home + "//work", ""},        // not clean
		{"work", ""},                 // not absolute
		{home + "/work/../.ssh", ""}, // not clean
		{"/HOME/x", ""},              // does not exist
		{"/tmp", "refused ("},
		{"/tmp/podman-run-1000", ""}, // may not exist
		{"/var/home", "refused ("},
		{"/run/user/1000", ""},
		{filepath.Join(home, "work", "proj"), "refused ("}, // ancestor of the dangling ~/.ssh target
		{filepath.Join(home, "work"), "refused ("},         // ancestor too
		{home, "refused ("},
	}
	for _, r := range rows {
		o := &runOptions{MountRoots: []string{r.src}}
		_, err := bindSource(o, r.src)
		if err == nil {
			t.Errorf("%-60q accepted", r.src)
			continue
		}
		if r.want != "" && !strings.Contains(err.Error(), r.want) && !strings.Contains(err.Error(), "is itself denied") {
			t.Errorf("%-60q refused for the wrong reason: %v", r.src, err)
		}
	}
	// the one acceptable row: a work dir that is neither ancestor nor descendant of any anchor
	_ = os.MkdirAll(filepath.Join(home, "elsewhere"), 0o700)
	ok := filepath.Join(home, "elsewhere")
	if _, err := bindSource(&runOptions{MountRoots: []string{ok}}, ok); err != nil {
		t.Errorf("unrelated work dir refused: %v", err)
	}
	o := &runOptions{remote: true, MountRoots: []string{"/srv2"}}
	if u, err := bindSource(o, "/srv2/x"); err != nil || u != "/srv2/x" {
		t.Errorf("remote /srv2/x -> %q err=%v", u, err)
	}
	// remote: home anchors are not known; /home as a root is itself denied
	o = &runOptions{remote: true, MountRoots: []string{"/home"}}
	if _, err := bindSource(o, "/home/u/.ssh"); err == nil || !strings.Contains(err.Error(), "is itself denied") {
		t.Errorf("remote /home root: %v", err)
	}
}
