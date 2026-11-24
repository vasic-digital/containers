# Forcing the container runner (`--runner`, `CONTAINERS_RUNNER`)

On a host with several container runners installed, the module's default
priority order (podman, docker, nerdctl, cri-o, lxd, kubernetes) picks the
runner. Every project in the ecosystem that has a `start.sh` / `restart.sh`
accepts one uniform override.

## Contract

| | |
|---|---|
| Flag | `--runner=NAME` or `--runner NAME` |
| Env | `CONTAINERS_RUNNER=NAME` |
| Precedence | flag > env > default priority order |
| `NAME` | one of the supported runners, or `auto` (= default order) |
| Supported runners | **derived from the module**: `runner list` / `runtime.SupportedRunners()` (today: podman, docker, nerdctl, cri-o, lxd, kubernetes). Never hardcode the list in a script. |
| Unknown `NAME` | refused, exit 2, message lists the supported runners (also when it comes from the env var) |
| Forced runner not installed / not reachable | refused, exit 2. There is NEVER a silent fallback to another runner |
| Compose | a forced runner only uses its own compose front-ends (docker: `docker compose`, `docker-compose`; podman: `podman-compose`, `podman compose`; nerdctl: `nerdctl compose`). Runners with no compose implementation are refused for compose use |

## Go API (`pkg/runtime`, `pkg/compose`)

```go
val, rest, found, err := runtime.ParseRunnerFlag(os.Args[1:])
choice, err := runtime.ResolveRunner(val, found, os.Getenv) // flag > env > default
rt, err := runtime.SelectRuntime(ctx, choice)               // forced => no fallback
orch, err := compose.NewDefaultOrchestratorForRunner(dir, logger, choice.Name)
```

Errors wrap `ErrBadRunnerFlag`, `ErrUnknownRunner`, `ErrRunnerUnavailable`.
`cmd/boot` accepts `--runner` natively.

## Shell helper (`scripts/lib/runner.sh`) for start/restart/stop scripts

```bash
source "$CONTAINERS_DIR/scripts/lib/runner.sh"   # CONTAINERS_DIR = the module checkout
runner_parse_args "$@" || exit 2                 # splits --runner out
set -- "${RUNNER_REST[@]+"${RUNNER_REST[@]}"}"   # the script's own args, flag removed
runner_select || exit 2                          # validates through the module
```

`runner_select` prints `runner: podman (selected by flag|env|default)` to stderr
and exports `CONTAINERS_RUNNER_CHOSEN`, `CONTAINERS_RUNNER_SOURCE`, and, only
when a runner was forced, `CONTAINERS_RUNNER`, so a `restart.sh` that calls
`start.sh`, or any Go tool using `runtime.ResolveRunner(.., os.Getenv)`,
honours the same choice. `runner_usage` prints a help paragraph whose runner list
comes from the module; `runner_compose_cmd` prints the compose command words.
The helper builds `cmd/runner` on demand into `~/.cache/containers-runner`
(or use a prebuilt one via `CONTAINERS_RUNNER_BIN`).

`cmd/runner` is the CLI behind it: `runner list`, `runner resolve [--runner N]`
(prints `NAME<TAB>SOURCE<TAB>forced|auto`), `runner compose [--runner N]`.

## Evidence / tests

* `go test ./pkg/runtime ./pkg/compose ./cmd/runner` includes real-host cases
  (this host: podman present, docker absent, so forcing docker is refused for real).
* `bash scripts/lib/runner.test.sh` real-host shell suite;
  `bash scripts/lib/runner.test.sh --prove-failure` runs paired mutations of
  `runner.sh` that the suite must kill.
