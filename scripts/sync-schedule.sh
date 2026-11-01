#!/usr/bin/env bash
# sync-schedule.sh: Thin wrapper around 'taite sync' (Go CLI).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

exec go run "${ROOT_DIR}/cmd/taite" sync -root "${ROOT_DIR}"
