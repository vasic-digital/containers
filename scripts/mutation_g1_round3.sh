#!/usr/bin/env bash
# Mutation run for the round-3 allowlist run builder (CAT-DEVEMU G1/G2/G11).
# Each mutant (table: scripts/mutants_g1_round3.py) edits ONE source file in
# place. It is first checked to COMPILE (go build + test-binary compile, both
# with -vet=off); a compile error is INVALID and never counted as a kill. Then
# the targeted tests run with -vet=off: KILLED = a test failed, SURVIVED = all
# passed (a gap). The file is restored after every mutant and verified with cmp.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT:?}" || exit 2
BK="$(mktemp -d)" || exit 2
: "${BK:?}"
export GOMAXPROCS=2
NICE="nice -n 10"
MUT=scripts/mutants_g1_round3.py
N="$(python3 "$MUT" count)"
killed=0; survived=0; invalid=0
for ((i=0;i<N;i++)); do
  file="$(python3 "$MUT" file "$i")"
  bk="$BK/orig"
  cp "$file" "$bk"
  out="$(python3 "$MUT" apply "$i" 2>&1)"; rc=$?
  if [ $rc -ne 0 ]; then
    echo "[$i] INVALID (mutation not applied): $out"; invalid=$((invalid+1)); cp "$bk" "$file"; continue
  fi
  pk="./pkg/runtime/"; run="."
  case "$file" in
    pkg/hostbudget/*) pk="./pkg/hostbudget/";;
    pkg/emulator/*) pk="./pkg/emulator/"; run='Probe|Reachable';;
  esac
  if ! $NICE go build ./... >/dev/null 2>&1 || ! $NICE go test -vet=off -count=1 -run '^$' "$pk" >/dev/null 2>&1; then
    echo "[$i] INVALID (does not compile): $out"; invalid=$((invalid+1))
  elif $NICE go test -vet=off -count=1 -p 1 -run "$run" "$pk" >/dev/null 2>&1; then
    echo "[$i] SURVIVED: $out"; survived=$((survived+1))
  else
    echo "[$i] killed: $out"; killed=$((killed+1))
  fi
  cp "$bk" "$file"
  if ! cmp -s "$file" "$bk"; then echo "RESTORE FAILED for $file"; exit 4; fi
done
rm -rf -- "${BK:?}"
echo "mutants=$N killed=$killed survived=$survived invalid=$invalid (restore cmp-verified after every mutant)"
[ "$survived" -eq 0 ] && [ "$invalid" -eq 0 ]
