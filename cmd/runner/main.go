// Command runner is the shell-callable front for the module's forced-runner
// selector (pkg/runtime/forced.go). start/restart scripts of every consuming
// project call it through scripts/lib/runner.sh so that the list of supported
// runners, the precedence rule and the availability check live in ONE place.
//
// Subcommands:
//
//	list                         supported runner names, one per line
//	resolve [--runner NAME]      validate + resolve; print "NAME<TAB>SOURCE<TAB>forced|auto"
//	compose [--runner NAME]      print the compose command words for the runner
//
// Precedence: --runner flag > $CONTAINERS_RUNNER > default priority order.
//
// Exit codes (three-valued; 2 is never a pass):
//
//	0 ok
//	1 usage error
//	2 refused: unknown runner, forced runner unavailable, or nothing available
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/runtime"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv)) }

func run(args []string, out, errw io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: runner list | resolve [--runner NAME] | compose [--runner NAME]")
		return 1
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		for _, n := range runtime.SupportedRunners() {
			fmt.Fprintln(out, n)
		}
		return 0
	case "resolve", "compose":
	default:
		fmt.Fprintf(errw, "runner: unknown subcommand %q\n", sub)
		return 1
	}
	val, extra, found, err := runtime.ParseRunnerFlag(rest)
	if err != nil {
		fmt.Fprintf(errw, "runner: %v\n", err)
		return 2
	}
	if len(extra) > 0 {
		fmt.Fprintf(errw, "runner: unexpected arguments: %s\n", strings.Join(extra, " "))
		return 1
	}
	choice, err := runtime.ResolveRunner(val, found, getenv)
	if err != nil {
		fmt.Fprintf(errw, "runner: %v\n", err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rt, err := runtime.SelectRuntime(ctx, choice)
	if err != nil {
		fmt.Fprintf(errw, "runner: %v\n", err)
		return 2
	}
	if sub == "resolve" {
		mode := "auto"
		if choice.Forced() {
			mode = "forced"
		}
		fmt.Fprintf(out, "%s\t%s\t%s\n", rt.Name(), choice.Source, mode)
		return 0
	}
	var cmd string
	var cargs []string
	if choice.Forced() {
		cmd, cargs, err = compose.DetectComposeCmdForRunner(rt.Name())
	} else {
		cmd, cargs, err = compose.DetectDefaultComposeCmd()
	}
	if err != nil {
		fmt.Fprintf(errw, "runner: %v\n", err)
		return 2
	}
	fmt.Fprintln(out, strings.Join(append([]string{cmd}, cargs...), " "))
	return 0
}
