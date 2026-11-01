#!/usr/bin/env bash
# verify.sh: Fast, unified mechanical verification for taite-board.
# Excludes deprecated iOS/Swift checks to ensure sub-3-second feedback.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=== [1/5] Syncing schedule copies ==="
"${SCRIPT_DIR}/sync-schedule.sh"

echo "=== [2/5] Validating JSON schema (taite validate) ==="
(cd "${ROOT_DIR}" && go run ./cmd/taite validate --file data/generated/schedule.json)

echo "=== [3/5] Running Web unit tests ==="
(cd "${ROOT_DIR}/apps/web" && npm test)

echo "=== [4/5] Running TypeScript typecheck ==="
(cd "${ROOT_DIR}/apps/web" && npm run typecheck)

echo "=== [5/5] Running Web bundle build ==="
(cd "${ROOT_DIR}/apps/web" && npm run build)

echo ""
echo "All verifications PASSED."
