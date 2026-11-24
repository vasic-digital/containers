package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Forced runner selection.
//
// A consumer start/restart script on a host with several container runners
// installed may need to override the default priority order (podman, docker,
// nerdctl, cri-o, lxd, kubernetes). The contract, shared by every consumer:
//
//	precedence: --runner=NAME flag  >  CONTAINERS_RUNNER env  >  default order
//
//   - NAME must be one of SupportedRunners() (derived from the module's own
//     runtime factory, never a second hardcoded list) or "auto", which means
//     "use the default priority order".
//   - An unknown NAME is an error (ErrUnknownRunner), whichever channel
//     supplied it.
//   - A forced NAME that is not installed or not reachable is an error
//     (ErrRunnerUnavailable). There is NEVER a silent fallback to another
//     runner: the operator asked for that one.

// EnvRunner is the environment variable that selects a runner when no flag was
// given.
const EnvRunner = "CONTAINERS_RUNNER"

// AutoRunner is the value that explicitly requests the default priority order.
const AutoRunner = "auto"

// Sources a RunnerChoice can come from.
const (
	SourceFlag    = "flag"
	SourceEnv     = "env"
	SourceDefault = "default"
)

var (
	// ErrUnknownRunner means the requested runner name is not supported.
	ErrUnknownRunner = errors.New("unknown container runner")
	// ErrRunnerUnavailable means the forced runner is not installed or not
	// reachable on this host.
	ErrRunnerUnavailable = errors.New("forced container runner is not available")
	// ErrBadRunnerFlag means the --runner flag itself was malformed.
	ErrBadRunnerFlag = errors.New("malformed --runner flag")
)

// RunnerChoice is a resolved selection. Name is empty when the default order
// applies.
type RunnerChoice struct {
	Name   string
	Source string
}

// Forced reports whether a specific runner was demanded.
func (c RunnerChoice) Forced() bool { return c.Name != "" }

// SupportedRunners lists every runner name the module can drive, in default
// priority order. It is derived from the runtime factory.
func SupportedRunners() []string {
	rts := defaultRuntimeFactory()
	names := make([]string, 0, len(rts))
	for _, rt := range rts {
		names = append(names, rt.Name())
	}
	return names
}

func supportedList() string { return strings.Join(SupportedRunners(), ", ") }

// ParseRunnerFlag extracts --runner=NAME / --runner NAME from args and returns
// the remaining arguments untouched. Arguments after a literal "--" are never
// inspected. Two --runner flags that disagree are an error.
func ParseRunnerFlag(args []string) (value string, rest []string, found bool, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		var v string
		switch {
		case a == "--runner":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return "", nil, false, fmt.Errorf("%w: --runner needs a value (one of: %s, %s)",
					ErrBadRunnerFlag, supportedList(), AutoRunner)
			}
			i++
			v = args[i]
		case strings.HasPrefix(a, "--runner="):
			v = strings.TrimPrefix(a, "--runner=")
			if v == "" {
				return "", nil, false, fmt.Errorf("%w: --runner= has an empty value", ErrBadRunnerFlag)
			}
		default:
			rest = append(rest, a)
			continue
		}
		if found && v != value {
			return "", nil, false, fmt.Errorf("%w: conflicting values %q and %q", ErrBadRunnerFlag, value, v)
		}
		value, found = v, true
	}
	return value, rest, found, nil
}

func normalizeRunner(raw, channel string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == AutoRunner {
		return "", nil
	}
	for _, n := range SupportedRunners() {
		if n == v {
			return n, nil
		}
	}
	return "", fmt.Errorf("%w %q (from %s): supported runners are %s, or %q for the default order",
		ErrUnknownRunner, raw, channel, supportedList(), AutoRunner)
}

// ResolveRunner applies precedence flag > env > default. flagSet says whether
// the flag was present; getenv is typically os.Getenv.
func ResolveRunner(flagValue string, flagSet bool, getenv func(string) string) (RunnerChoice, error) {
	if flagSet {
		n, err := normalizeRunner(flagValue, "--runner")
		if err != nil {
			return RunnerChoice{}, err
		}
		return RunnerChoice{Name: n, Source: SourceFlag}, nil
	}
	if getenv != nil {
		if raw := strings.TrimSpace(getenv(EnvRunner)); raw != "" {
			n, err := normalizeRunner(raw, "$"+EnvRunner)
			if err != nil {
				return RunnerChoice{}, err
			}
			return RunnerChoice{Name: n, Source: SourceEnv}, nil
		}
	}
	return RunnerChoice{Source: SourceDefault}, nil
}

// SelectRuntime returns the runtime for a choice. A forced choice returns that
// runtime or ErrRunnerUnavailable; it never falls back. The default choice is
// AutoDetect.
func SelectRuntime(ctx context.Context, c RunnerChoice) (ContainerRuntime, error) {
	if !c.Forced() {
		return AutoDetect(ctx)
	}
	for _, rt := range detectFactory() {
		if rt.Name() != c.Name {
			continue
		}
		if rt.IsAvailable(ctx) {
			return rt, nil
		}
		return nil, fmt.Errorf("%w: %q (selected via %s) is not installed or not reachable on this host; "+
			"refusing to fall back to another runner", ErrRunnerUnavailable, c.Name, c.Source)
	}
	return nil, fmt.Errorf("%w %q", ErrUnknownRunner, c.Name)
}
