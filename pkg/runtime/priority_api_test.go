//go:build !integration

package runtime

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuntimePriority_NotExported is the RT-RACE-1 regression guard
// (containers-2.0.0). The priority list used to be the exported package var
// RuntimePriority, which any caller could read or assign directly, bypassing
// priorityMu: a genuine data race against GetRuntimePriority /
// SetRuntimePriority that no lock inside this package can prevent. The only
// complete fix is to make the variable unreachable from outside the package,
// so the guard parses the package's non-test sources and fails if any
// exported package-level identifier names the priority list again.
//
// Control needle (§11.4.273): the same walk MUST find the exported accessor
// GetRuntimePriority, proving the walker really sees exported declarations.
func TestRuntimePriority_NotExported(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)
	pkg, ok := pkgs["runtime"]
	require.True(t, ok, "package runtime must be parsed")

	exportedVars := map[string]bool{}
	exportedFuncs := map[string]bool{}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			switch dd := d.(type) {
			case *ast.GenDecl:
				if dd.Tok != token.VAR {
					continue
				}
				for _, sp := range dd.Specs {
					for _, n := range sp.(*ast.ValueSpec).Names {
						if n.IsExported() {
							exportedVars[n.Name] = true
						}
					}
				}
			case *ast.FuncDecl:
				if dd.Recv == nil && dd.Name.IsExported() {
					exportedFuncs[dd.Name.Name] = true
				}
			}
		}
	}

	assert.True(t, exportedFuncs["GetRuntimePriority"], "control needle: walker must see exported funcs")
	assert.True(t, exportedFuncs["SetRuntimePriority"], "control needle: walker must see exported funcs")
	assert.False(t, exportedVars["RuntimePriority"],
		"RuntimePriority must not be an exported package var (RT-RACE-1): use Get/SetRuntimePriority")
}

// TestRuntimePriority_ConcurrentAccessorsAreRaceFree hammers the only
// remaining access path from many goroutines; under `go test -race` any
// unsynchronised access to the backing list fails the run.
func TestRuntimePriority_ConcurrentAccessorsAreRaceFree(t *testing.T) {
	orig := GetRuntimePriority()
	t.Cleanup(func() { SetRuntimePriority(orig) })

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				SetRuntimePriority([]string{"docker", "podman"})
			} else {
				SetRuntimePriority([]string{"podman", "docker", "nerdctl"})
			}
		}(i)
		go func() {
			defer wg.Done()
			got := GetRuntimePriority()
			assert.NotEmpty(t, got)
		}()
	}
	wg.Wait()
}

// TestRuntimePriority_GetReturnsCopy proves callers cannot mutate the
// internal list through the returned slice (the property the exported var
// could not give).
func TestRuntimePriority_GetReturnsCopy(t *testing.T) {
	orig := GetRuntimePriority()
	t.Cleanup(func() { SetRuntimePriority(orig) })

	SetRuntimePriority([]string{"podman", "docker"})
	got := GetRuntimePriority()
	got[0] = "tampered"
	assert.Equal(t, []string{"podman", "docker"}, GetRuntimePriority())

	in := []string{"docker", "podman"}
	SetRuntimePriority(in)
	in[0] = "tampered"
	assert.Equal(t, []string{"docker", "podman"}, GetRuntimePriority())
}
