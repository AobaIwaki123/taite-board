#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=== Building & Syncing taite-board artifacts ==="

# 1. Ensure target directories exist
mkdir -p "${REPO_ROOT}/docs"
mkdir -p "${REPO_ROOT}/web"
mkdir -p "${REPO_ROOT}/docs/images"
mkdir -p "${REPO_ROOT}/web/images"

# 2. Sync apps/web source (HTML, JS, CSS, images) to docs/ and web/
cp "${REPO_ROOT}/apps/web/index.html" "${REPO_ROOT}/docs/index.html"
cp "${REPO_ROOT}/apps/web/index.html" "${REPO_ROOT}/web/index.html"

if [ -f "${REPO_ROOT}/apps/web/app.js" ]; then
  cp "${REPO_ROOT}/apps/web/app.js" "${REPO_ROOT}/docs/app.js"
  cp "${REPO_ROOT}/apps/web/app.js" "${REPO_ROOT}/web/app.js"
fi

if [ -f "${REPO_ROOT}/apps/web/styles.css" ]; then
  cp "${REPO_ROOT}/apps/web/styles.css" "${REPO_ROOT}/docs/styles.css"
  cp "${REPO_ROOT}/apps/web/styles.css" "${REPO_ROOT}/web/styles.css"
fi

if [ -d "${REPO_ROOT}/apps/web/images" ]; then
  cp -r "${REPO_ROOT}/apps/web/images/"* "${REPO_ROOT}/docs/images/" || true
  cp -r "${REPO_ROOT}/apps/web/images/"* "${REPO_ROOT}/web/images/" || true
fi

# 3. Sync data/generated/schedule.json to docs/ and web/ (both as schedule.json and mock_schedule.json for backward compatibility)
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/docs/schedule.json"
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/web/schedule.json"
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/docs/mock_schedule.json"
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/web/mock_schedule.json"
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/data/mock_schedule.json"

echo "=== Artifacts synced successfully! ==="
