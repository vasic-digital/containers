#!/usr/bin/env bash
# Installs the anti-bluff pre-commit hook into .git/hooks/pre-commit.
# Idempotent.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

HOOK_TARGET="${ROOT_DIR}/.git/hooks/pre-commit"
HOOK_SOURCE="${SCRIPT_DIR}/pre-commit-hook.sh"

# .git may be a file (submodule) pointing at the real gitdir.
if [[ -f "${ROOT_DIR}/.git" ]]; then
  GITDIR_LINE="$(head -n1 "${ROOT_DIR}/.git")"
  if [[ "${GITDIR_LINE}" =~ ^gitdir:\ (.*) ]]; then
    REAL_GITDIR="${BASH_REMATCH[1]}"
    case "${REAL_GITDIR}" in
      /*) : ;;
      *) REAL_GITDIR="${ROOT_DIR}/${REAL_GITDIR}" ;;
    esac
    HOOK_TARGET="${REAL_GITDIR}/hooks/pre-commit"
    mkdir -p "$(dirname "${HOOK_TARGET}")"
  fi
fi

if [[ -e "${HOOK_TARGET}" && ! -L "${HOOK_TARGET}" ]]; then
  echo "Existing non-symlink pre-commit hook at ${HOOK_TARGET}; refusing to overwrite." >&2
  echo "Move it aside, then re-run." >&2
  exit 1
fi

ln -sf "${HOOK_SOURCE}" "${HOOK_TARGET}"
chmod +x "${HOOK_SOURCE}"
echo "Installed ${HOOK_TARGET} -> ${HOOK_SOURCE}"
