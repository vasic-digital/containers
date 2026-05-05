#!/usr/bin/env bash
# CONST-035 mutation ratchet challenge (Go).
# Modes:
#   default (no --mode): run on changed files vs main.
#   --mode all: run full project (slow).
# Compares against challenges/baselines/bluff-baseline.txt Section 2.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MODE="changed"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 3 ;;
  esac
done

BASELINE="${ROOT_DIR}/challenges/baselines/bluff-baseline.txt"
CONFIG="${ROOT_DIR}/.go-mutesting.yml"

if ! command -v go-mutesting >/dev/null; then
  GOPATH_BIN="$(go env GOPATH)/bin"
  export PATH="${GOPATH_BIN}:${PATH}"
fi

if ! command -v go-mutesting >/dev/null; then
  echo "FAIL: go-mutesting not installed (run: go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest)" >&2
  exit 1
fi

OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

if [[ "$MODE" == "changed" ]]; then
  mapfile -t CHANGED < <(git -C "${ROOT_DIR}" diff --name-only main...HEAD -- '*.go')
  if (( ${#CHANGED[@]} == 0 )); then
    echo "OK: no Go changes vs main."
    exit 0
  fi
  PKGS=()
  for f in "${CHANGED[@]}"; do PKGS+=("./$(dirname "$f")"); done
  PKGS=($(printf '%s\n' "${PKGS[@]}" | sort -u))
  ( cd "${ROOT_DIR}" && go-mutesting --config="${CONFIG}" "${PKGS[@]}" ) > "${OUT}" 2>&1 || true
else
  ( cd "${ROOT_DIR}" && go-mutesting --config="${CONFIG}" ./... ) > "${OUT}" 2>&1 || true
fi

# Parse per-file kill rates (regex must match installed go-mutesting version).
python3 - "${OUT}" "${BASELINE}" "${MODE}" <<'PYEOF'
import collections, re, sys
out_path, baseline_path, mode = sys.argv[1], sys.argv[2], sys.argv[3]
killed = collections.Counter(); total = collections.Counter()
# Avito-fork output lines look like:
#   PASS "/tmp/.../<repo>-go-mutesting-NNN/path/to/file.go.NN" with checksum ...
#   FAIL "/tmp/.../<repo>-go-mutesting-NNN/path/to/file.go.NN" with checksum ...
# Strip the temp-dir prefix and the trailing .NN mutation index to recover the
# repo-relative path.
line_re = re.compile(r'^(PASS|FAIL)\s+"([^"]+\.go)\.(\d+)"')
with open(out_path) as f:
    for line in f:
        m = line_re.match(line)
        if not m: continue
        verdict, full, _idx = m.group(1), m.group(2), m.group(3)
        # Strip leading temp dir; keep the path under the repo root.
        # Find first occurrence of "/" + first known top-level dir; conservative:
        # just take the path after the second '/' inside /tmp/ or rely on a marker.
        # Easiest robust strategy: drop the first 4 path components (e.g.,
        # "/tmp/.private/USER/go-mutesting-NNN/").
        parts = full.split('/')
        try:
            anchor = next(i for i, p in enumerate(parts) if p.startswith('go-mutesting-'))
            repo_rel = '/'.join(parts[anchor+1:])
        except StopIteration:
            repo_rel = full
        total[repo_rel] += 1
        if verdict == "PASS":
            killed[repo_rel] += 1

baseline = {}
section = None
with open(baseline_path) as f:
    for line in f:
        line = line.rstrip("\n")
        if line.startswith("# === SECTION 2"): section = 2; continue
        if line.startswith("# === SECTION 3"): section = 3; continue
        if section == 2 and line and not line.startswith("#"):
            try:
                p, r, n = line.split(":")
                baseline[p] = int(r)
            except ValueError: continue

failed = False
for fn in sorted(total.keys()):
    rate = 100 * killed[fn] // total[fn] if total[fn] else 0
    if mode == "changed" and rate < 90:
        print(f"FAIL: {fn} kill rate {rate}% < 90% (changed-code threshold)")
        failed = True
    if fn in baseline and rate < baseline[fn]:
        print(f"FAIL: {fn} kill rate {rate}% < baseline {baseline[fn]}% (ratchet)")
        failed = True

if mode == "all":
    overall_killed = sum(killed.values()); overall_total = sum(total.values())
    overall_rate = 100 * overall_killed // overall_total if overall_total else 0
    if overall_rate < 80:
        print(f"WARN: project-wide kill rate {overall_rate}% < 80% target (sub-project 1 baseline; ratchet enforces non-regression).")

sys.exit(1 if failed else 0)
PYEOF

echo "OK: mutation ratchet (mode=${MODE})."
