#!/usr/bin/env bash
# scripts/lib/runner.test.sh -- real-host test of runner.sh against the real
# module and the real runtimes of THIS host. No mocks.
#   bash scripts/lib/runner.test.sh                 run the suite
#   bash scripts/lib/runner.test.sh --prove-failure  run the paired mutations: each
#                                                    mutated copy of runner.sh MUST
#                                                    make the suite fail (rc 0 = all
#                                                    mutations were caught)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="${RUNNER_LIB_UNDER_TEST:-$HERE/runner.sh}"
export CONTAINERS_DIR="$(cd "$HERE/../.." && pwd)"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  ok   $*"; }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL $*"; }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2', want '$3')"; fi; }

run_suite() {
  # shellcheck disable=SC1090
  source "$LIB"
  local have_podman=0 have_docker=0
  command -v podman >/dev/null 2>&1 && have_podman=1
  command -v docker >/dev/null 2>&1 && have_docker=1

  # list comes from the module
  local l; l="$(runner_list | paste -sd, -)"
  case "$l" in podman,docker,nerdctl*) ok "list from module: $l" ;; *) bad "list: $l" ;; esac

  # parse
  runner_parse_args a --runner=docker b --runner podman 2>/dev/null; check "conflict rc" "$?" 2
  runner_parse_args a --runner podman b -- --runner=x; check "parse rc" "$?" 0
  check "parse value" "$RUNNER_FLAG_VALUE" podman
  check "parse rest" "${RUNNER_REST[*]}" "a b -- --runner=x"
  runner_parse_args --runner 2>/dev/null; check "missing value rc" "$?" 2
  runner_parse_args --runner= 2>/dev/null; check "empty value rc" "$?" 2

  # unknown runner refused (rc 2), nothing exported
  ( unset CONTAINERS_RUNNER CONTAINERS_RUNNER_CHOSEN; runner_parse_args --runner=rkt; runner_select 2>/dev/null; echo "$? ${CONTAINERS_RUNNER_CHOSEN:-none}" ) > "$TMPOUT"
  check "unknown runner refused, nothing chosen" "$(cat "$TMPOUT")" "2 none"

  # env unknown refused too
  ( export CONTAINERS_RUNNER=bogus; runner_parse_args; runner_select 2>/dev/null; echo $? ) > "$TMPOUT"
  check "env unknown refused" "$(cat "$TMPOUT")" 2

  if [ "$have_docker" = 0 ]; then
    ( unset CONTAINERS_RUNNER; runner_parse_args --runner=docker; runner_select 2>"$TMPERR"; echo "$? ${CONTAINERS_RUNNER_CHOSEN:-none}" ) > "$TMPOUT"
    check "forced absent docker refused, no fallback" "$(cat "$TMPOUT")" "2 none"
    ( export CONTAINERS_RUNNER=docker; runner_parse_args; runner_select 2>/dev/null; echo "$? ${CONTAINERS_RUNNER_CHOSEN:-none}" ) > "$TMPOUT"
    check "env-forced absent docker refused, no fallback" "$(cat "$TMPOUT")" "2 none"
  else echo "  SKIP absent-docker cases: docker installed on this host"; fi

  if [ "$have_podman" = 1 ]; then
    ( unset CONTAINERS_RUNNER; runner_parse_args --runner=podman; runner_select 2>"$TMPERR"; echo "$? $CONTAINERS_RUNNER_CHOSEN $CONTAINERS_RUNNER_SOURCE $CONTAINERS_RUNNER" ) > "$TMPOUT"
    check "flag podman exports" "$(cat "$TMPOUT")" "0 podman flag podman"
    grep -q 'runner: podman (selected by flag)' "$TMPERR" && ok "announces chosen runner and how" || bad "announce: $(cat "$TMPERR")"
    ( export CONTAINERS_RUNNER=podman; runner_parse_args; runner_select 2>/dev/null; echo "$CONTAINERS_RUNNER_SOURCE" ) > "$TMPOUT"
    check "env source" "$(cat "$TMPOUT")" env
    ( export CONTAINERS_RUNNER=docker; runner_parse_args --runner=podman; runner_select 2>/dev/null; echo "$? $CONTAINERS_RUNNER_SOURCE $CONTAINERS_RUNNER" ) > "$TMPOUT"
    check "flag beats env" "$(cat "$TMPOUT")" "0 flag podman"
    ( unset CONTAINERS_RUNNER; runner_parse_args; runner_select 2>/dev/null; echo "$? $CONTAINERS_RUNNER_SOURCE ${CONTAINERS_RUNNER:-unset}" ) > "$TMPOUT"
    check "default leaves CONTAINERS_RUNNER unset" "$(cat "$TMPOUT")" "0 default unset"
    ( export CONTAINERS_RUNNER=docker; runner_parse_args --runner=auto; runner_select 2>/dev/null; echo "$? ${CONTAINERS_RUNNER:-unset}" ) > "$TMPOUT"
    check "--runner=auto clears inherited env" "$(cat "$TMPOUT")" "0 unset"
    ( unset CONTAINERS_RUNNER; runner_parse_args --runner=auto; runner_select 2>/dev/null; echo "$? ${CONTAINERS_RUNNER:-unset}" ) > "$TMPOUT"
    check "auto = default order" "$(cat "$TMPOUT")" "0 unset"
  else echo "  SKIP podman cases: podman not installed"; fi
  runner_usage | grep -q 'podman|docker' && ok "usage lists runners from module" || bad "usage"
}

TMPOUT="$(mktemp)"; TMPERR="$(mktemp)"; trap 'rm -f "$TMPOUT" "$TMPERR"' EXIT

if [ "${1:-}" = "--prove-failure" ]; then
  # Each mutation edits a COPY of runner.sh; the suite on the copy must FAIL.
  d="$(mktemp -d)"; trap 'rm -rf "$d" "$TMPOUT" "$TMPERR"' EXIT
  mut() { # name sed-expr
    cp "$HERE/runner.sh" "$d/runner.sh"; sed -i -E "$2" "$d/runner.sh"
    if cmp -s "$HERE/runner.sh" "$d/runner.sh"; then echo "MUTATION $1: sed made no change (invalid mutant)"; INVALID=1; return; fi
    if RUNNER_LIB_UNDER_TEST="$d/runner.sh" CONTAINERS_DIR="$CONTAINERS_DIR" bash "$HERE/runner.test.sh" >/dev/null 2>&1; then
      echo "MUTATION $1: SURVIVED (suite stayed green)"; SURV=1; else echo "MUTATION $1: killed"; fi
  }
  SURV=0; INVALID=0
  # control: the unmutated suite must be green
  bash "$HERE/runner.test.sh" >/dev/null 2>&1 && echo "control: green" || { echo "control: RED"; exit 1; }
  mut M1-swallow-refusal 's/out="\$\("\$b" resolve --runner "\$RUNNER_FLAG_VALUE"\)" \|\| return 2/out="$("$b" resolve --runner "$RUNNER_FLAG_VALUE" || "$b" resolve)"/'
  mut M2-env-beats-flag 's/if \[ "\$RUNNER_FLAG_SET" = 1 \]; then\n//; s/RUNNER_FLAG_SET=1$/RUNNER_FLAG_SET=0/'
  mut M3-always-export 's/if \[ "\$mode" = forced \]; then export/if true; then export/'
  mut M4-no-conflict-check 's/\[ "\$RUNNER_FLAG_VALUE" != "\$1" \]/false/'
  mut M5-no-announce "s/printf 'runner: %s \(selected by %s\)\\\\n' \"\\\$name\" \"\\\$source\" >&2/:/"
  mut M6-hardcoded-list "s/\"\\\$b\" list;/printf 'podman\\\\ndocker\\\\n';/"
  mut M8-auto-leaks-env 's/elif \[ "\$RUNNER_FLAG_SET" = 1 \]; then unset CONTAINERS_RUNNER; fi/elif false; then :; fi/'
  mut M7-rest-dropped 's/\*\) RUNNER_REST\+=\("\$1"\); shift ;;/*) shift ;;/'
  [ "$SURV" = 0 ] && [ "$INVALID" = 0 ] && exit 0 || exit 1
fi

echo "runner.sh real-host suite ($LIB)"
run_suite
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" = 0 ]
