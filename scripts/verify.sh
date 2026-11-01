#!/usr/bin/env bash
# verify.sh: Fast, unified mechanical verification for taite-board.
# Excludes deprecated iOS/Swift checks to ensure sub-3-second feedback.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=== [1/6] Syncing schedule copies ==="
"${SCRIPT_DIR}/sync-schedule.sh"

echo "=== [2/6] Validating JSON schema (taite validate) ==="
(cd "${ROOT_DIR}" && go run ./cmd/taite validate --file data/generated/schedule.json)

echo "=== [3/6] Running Web unit tests ==="
(cd "${ROOT_DIR}/apps/web" && npm test)

echo "=== [4/6] Running TypeScript typecheck ==="
(cd "${ROOT_DIR}/apps/web" && npm run typecheck)

echo "=== [5/6] Running Web bundle build ==="
(cd "${ROOT_DIR}/apps/web" && npm run build)

echo "=== [6/6] Verifying typography floor standard (>= 12px) ==="
"${SCRIPT_DIR}/verify-typography.sh"

echo ""
echo "All verifications PASSED."
