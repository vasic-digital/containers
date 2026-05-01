#!/usr/bin/env bash
# Wraps bluff-scanner.sh as a challenge.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
exec bash "${ROOT_DIR}/scripts/anti-bluff/bluff-scanner.sh" --mode all
