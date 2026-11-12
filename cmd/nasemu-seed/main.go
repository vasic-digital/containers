// Command nasemu-seed generates a deterministic seed corpus with its golden
// manifest and verifies live trees against that manifest. PROVISIONAL.
//
//	nasemu-seed generate -spec spec.json -out DIR -manifest-out m.json [-evidence-dir D -label L]
//	nasemu-seed verify   -manifest m.json -manifest-sha256 HEX -root DIR [-mode] [-mtime] [-skip-symlinks] [-evidence-dir D -label L]
//
// Exit codes: 0 PASS, 1 FAIL, 2 ERROR (could not determine), 64 usage.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"digital.vasic.containers/pkg/nasemu/seed"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitError = 2
	exitUsage = 64
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: nasemu-seed generate|verify [flags]")
		return exitUsage
	}
	switch args[0] {
	case "generate":
		return generate(args[1:], out, errw)
	case "verify":
		return verify(args[1:], out, errw)
	}
	fmt.Fprintf(errw, "unknown command %q\n", args[0])
	return exitUsage
}

func generate(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(errw)
	specPath := fs.String("spec", "", "spec JSON file")
	dir := fs.String("out", "", "tree output directory (absent or empty)")
	mpath := fs.String("manifest-out", "", "manifest output file (must not exist)")
	evDir := fs.String("evidence-dir", "", "write evidence JSON here")
	label := fs.String("label", "", "evidence label")
	if fs.Parse(args) != nil || *specPath == "" || *dir == "" || *mpath == "" || (*evDir != "") != (*label != "") {
		fmt.Fprintln(errw, "generate: -spec, -out, -manifest-out required; -evidence-dir and -label go together")
		return exitUsage
	}
	raw, err := os.ReadFile(*specPath)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	spec, err := seed.ParseSpec(raw)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	m, err := seed.Generate(spec, *dir)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	mb, err := seed.MarshalManifest(m)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	f, err := os.OpenFile(*mpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	_, werr := f.Write(mb)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		fmt.Fprintln(errw, werr)
		return exitError
	}
	h := seed.ManifestSHA256(mb)
	fmt.Fprintf(out, "manifest_sha256=%s\n", h)
	if *evDir != "" {
		ev, err := seed.NewGenerateEvidence(*label, m.SpecSHA256, h, m)
		if err == nil {
			_, err = seed.WriteEvidence(*evDir, ev)
		}
		if err != nil {
			fmt.Fprintln(errw, err)
			return exitError
		}
	}
	return exitPass
}

func verify(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(errw)
	mpath := fs.String("manifest", "", "manifest file")
	want := fs.String("manifest-sha256", "", "expected manifest sha256 (required)")
	root := fs.String("root", "", "tree to verify")
	mode := fs.Bool("mode", false, "also compare permission bits")
	mtime := fs.Bool("mtime", false, "also compare mtimes")
	skipSym := fs.Bool("skip-symlinks", false, "do not check symlink entries (counted as skipped)")
	evDir := fs.String("evidence-dir", "", "write evidence JSON here")
	label := fs.String("label", "", "evidence label")
	if fs.Parse(args) != nil || *mpath == "" || *root == "" || (*evDir != "") != (*label != "") {
		fmt.Fprintln(errw, "verify: -manifest, -manifest-sha256, -root required; -evidence-dir and -label go together")
		return exitUsage
	}
	mb, err := os.ReadFile(*mpath)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	m, err := seed.ParseManifest(mb, *want)
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	rep, err := seed.VerifyDir(*root, m, seed.Options{CheckMode: *mode, CheckMtime: *mtime, SkipSymlinks: *skipSym})
	if err != nil {
		fmt.Fprintln(errw, err)
		return exitError
	}
	fmt.Fprintf(out, "result=%s diffs=%d\n", rep.Result, len(rep.Diffs))
	for _, d := range rep.Diffs {
		fmt.Fprintf(out, "%s\t%s\t%s\n", d.Code, d.Path, d.Detail)
	}
	if *evDir != "" {
		ev, err := seed.NewVerifyEvidence(*label, seed.ManifestSHA256(mb), rep)
		if err == nil {
			_, err = seed.WriteEvidence(*evDir, ev)
		}
		if err != nil {
			fmt.Fprintln(errw, err)
			return exitError
		}
	}
	switch rep.Result {
	case seed.ResultPass:
		return exitPass
	case seed.ResultFail:
		return exitFail
	}
	return exitError
}
