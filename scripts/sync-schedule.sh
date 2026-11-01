#!/usr/bin/env bash
# sync-schedule.sh: Synchronize data/generated/schedule.json to web public directories.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

SOURCE="${ROOT_DIR}/data/generated/schedule.json"
TARGET_WEB="${ROOT_DIR}/apps/web/public/schedule.json"
TARGET_MOCK="${ROOT_DIR}/apps/web/public/mock_schedule.json"

if [[ ! -f "${SOURCE}" ]]; then
  echo "Error: Source schedule file not found: ${SOURCE}" >&2
  exit 1
fi

cp "${SOURCE}" "${TARGET_WEB}"
cp "${SOURCE}" "${TARGET_MOCK}"

# Verify identical checksums
SRC_HASH=$(shasum -a 256 "${SOURCE}" | awk '{print $1}')
WEB_HASH=$(shasum -a 256 "${TARGET_WEB}" | awk '{print $1}')
MOCK_HASH=$(shasum -a 256 "${TARGET_MOCK}" | awk '{print $1}')

if [[ "${SRC_HASH}" != "${WEB_HASH}" || "${SRC_HASH}" != "${MOCK_HASH}" ]]; then
  echo "Error: Checksum mismatch after sync!" >&2
  exit 1
fi

echo "Synced schedule.json -> apps/web/public/ (hash: ${SRC_HASH:0:8})"
