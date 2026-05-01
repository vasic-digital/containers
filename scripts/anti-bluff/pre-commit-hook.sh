#!/usr/bin/env bash
# Pre-commit hook — runs scanner + manifest check on staged files.
# Mutation gate is excluded (too slow for pre-commit).
set -euo pipefail
# Resolve through symlinks so that when this hook is installed via
# `ln -s` into .git/hooks/pre-commit (or the submodule's
# .git/modules/<name>/hooks/pre-commit) the SCRIPT_DIR still points at
# the real scripts/anti-bluff/ directory.
SOURCE="${BASH_SOURCE[0]}"
while [[ -L "$SOURCE" ]]; do
  SOURCE_DIR="$(cd "$(dirname "$SOURCE")" && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ "$SOURCE" != /* ]] && SOURCE="${SOURCE_DIR}/${SOURCE}"
done
SCRIPT_DIR="$(cd "$(dirname "$SOURCE")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Run scanner in changed-mode against staged files only.
"${SCRIPT_DIR}/bluff-scanner.sh" --mode changed

# Run anchor manifest check (cheap, < 1s).
if [[ -f "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh" ]]; then
  bash "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh"
fi
