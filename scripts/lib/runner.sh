#!/usr/bin/env bash
# scripts/lib/runner.sh -- forced container-runner selection for start/restart/stop
# scripts. SOURCE this file; it has no top-level side effects.
#
# CONTRACT (the same for every project in the ecosystem):
#
#   precedence    --runner=NAME | --runner NAME   >   $CONTAINERS_RUNNER   >   default order
#   NAME          one of `runner list` (derived from pkg/runtime, never hardcoded
#                 here or in any consumer) or `auto` (= default order)
#   unknown NAME  -> refusal, rc 2
#   forced NAME not installed / not reachable -> refusal, rc 2. NEVER a silent
#                 fallback to another runner.
#
# API
#   runner_parse_args "$@"   splits the flag out. Sets RUNNER_REST (array: the
#                            remaining args, order kept), RUNNER_FLAG_VALUE,
#                            RUNNER_FLAG_SET (0|1). Returns 2 on a malformed flag.
#   runner_select            validates + resolves through the module, prints
#                            "runner: NAME (selected by flag|env|default)" to
#                            stderr and exports:
#                              CONTAINERS_RUNNER_CHOSEN  concrete runner name
#                              CONTAINERS_RUNNER_SOURCE  flag|env|default
#                              CONTAINERS_RUNNER         ONLY when forced, so every
#                                                        child (restart -> start,
#                                                        site-build, workshop-boot)
#                                                        honours the same choice.
#                            Returns 2 on refusal (caller must exit with it).
#   runner_list              supported runner names, one per line (from the module)
#   runner_usage             a ready-made help paragraph listing them
#   runner_compose_cmd       compose command words for the chosen runner
#
# Typical use (see docs/RUNNER_FLAG.md):
#   source "$CONTAINERS_DIR/scripts/lib/runner.sh"
#   runner_parse_args "$@" || exit 2
#   set -- "${RUNNER_REST[@]+"${RUNNER_REST[@]}"}"
#   runner_select || exit 2
#
# Env knobs: CONTAINERS_DIR (module checkout; auto-derived when this file is
# sourced from inside it), CONTAINERS_RUNNER_BIN (prebuilt `runner` binary).

_runner_lib_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${CONTAINERS_DIR:=$(cd "$_runner_lib_dir/../.." && pwd)}"

_runner_err() { printf 'runner: %s\n' "$*" >&2; }

# _runner_bin -- echo the path of the module's `runner` binary, building it on
# demand into the user cache (never into a tracked tree). rc 2 when it cannot.
_runner_bin() {
  if [ -n "${CONTAINERS_RUNNER_BIN:-}" ]; then
    [ -x "$CONTAINERS_RUNNER_BIN" ] || { _runner_err "CONTAINERS_RUNNER_BIN=$CONTAINERS_RUNNER_BIN is not executable"; return 2; }
    printf '%s\n' "$CONTAINERS_RUNNER_BIN"; return 0
  fi
  local cache="${XDG_CACHE_HOME:-$HOME/.cache}/containers-runner" bin
  bin="$cache/runner"
  if [ -x "$bin" ] && [ -z "$(find "$CONTAINERS_DIR/cmd/runner" "$CONTAINERS_DIR/pkg/runtime" "$CONTAINERS_DIR/pkg/compose" -name '*.go' -newer "$bin" 2>/dev/null | head -1)" ]; then
    printf '%s\n' "$bin"; return 0
  fi
  command -v go >/dev/null 2>&1 || { _runner_err "go toolchain not found and no prebuilt runner binary (set CONTAINERS_RUNNER_BIN)"; return 2; }
  mkdir -p "$cache" || { _runner_err "cannot create $cache"; return 2; }
  ( cd "$CONTAINERS_DIR" && go build -o "$bin.tmp.$$" ./cmd/runner ) >&2 \
    || { rm -f "$bin.tmp.$$"; _runner_err "go build ./cmd/runner failed in $CONTAINERS_DIR"; return 2; }
  mv -f "$bin.tmp.$$" "$bin"
  printf '%s\n' "$bin"
}

runner_list() { local b; b="$(_runner_bin)" || return 2; "$b" list; }

runner_usage() {
  local names
  names="$(runner_list 2>/dev/null | paste -sd'|' -)" || names="(unavailable)"
  printf '  --runner=NAME   force the container runner (%s|auto); overrides $CONTAINERS_RUNNER\n' "$names"
  printf '                  and the default priority order. An unknown or not-installed runner is refused (rc 2).\n'
}

runner_parse_args() {
  RUNNER_REST=(); RUNNER_FLAG_VALUE=""; RUNNER_FLAG_SET=0
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --) RUNNER_REST+=("$@"); break ;;
      --runner)
        if [ "$#" -lt 2 ] || [ "${2#-}" != "$2" ]; then _runner_err "--runner needs a value"; return 2; fi
        _runner_set "$2" || return 2; shift 2 ;;
      --runner=*)
        [ -n "${1#--runner=}" ] || { _runner_err "--runner= has an empty value"; return 2; }
        _runner_set "${1#--runner=}" || return 2; shift ;;
      *) RUNNER_REST+=("$1"); shift ;;
    esac
  done
  return 0
}

_runner_set() {
  if [ "$RUNNER_FLAG_SET" = 1 ] && [ "$RUNNER_FLAG_VALUE" != "$1" ]; then
    _runner_err "conflicting --runner values '$RUNNER_FLAG_VALUE' and '$1'"; return 2
  fi
  RUNNER_FLAG_VALUE="$1"; RUNNER_FLAG_SET=1
}

runner_select() {
  local b out name source mode rest
  b="$(_runner_bin)" || return 2
  if [ "$RUNNER_FLAG_SET" = 1 ]; then
    out="$("$b" resolve --runner "$RUNNER_FLAG_VALUE")" || return 2
  else
    out="$("$b" resolve)" || return 2
  fi
  name="${out%%$'\t'*}"; rest="${out#*$'\t'}"; source="${rest%%$'\t'*}"; mode="${rest#*$'\t'}"
  [ -n "$name" ] && [ -n "$source" ] && [ -n "$mode" ] || { _runner_err "unparseable answer from $b: '$out'"; return 2; }
  export CONTAINERS_RUNNER_CHOSEN="$name" CONTAINERS_RUNNER_SOURCE="$source"
  if [ "$mode" = forced ]; then export CONTAINERS_RUNNER="$name"
  elif [ "$RUNNER_FLAG_SET" = 1 ]; then unset CONTAINERS_RUNNER; fi   # explicit auto beats an inherited env
  printf 'runner: %s (selected by %s)\n' "$name" "$source" >&2
  return 0
}

runner_compose_cmd() {
  local b; b="$(_runner_bin)" || return 2
  if [ -n "${CONTAINERS_RUNNER_CHOSEN:-}" ]; then "$b" compose --runner "$CONTAINERS_RUNNER_CHOSEN"; else "$b" compose; fi
}
