#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=== Building Ribbon Clock Modern TypeScript Frontend ==="

# 1. Build apps/web via Vite
cd "${REPO_ROOT}/apps/web"
if [ ! -d "node_modules" ]; then
  echo "Installing web dependencies..."
  npm ci
fi
npm run build

echo "=== Syncing artifacts to docs/ (GitHub Pages fallback) ==="

# 2. Sync to docs/ without touching documentation markdown files
mkdir -p "${REPO_ROOT}/docs"
rm -rf "${REPO_ROOT}/docs/assets" "${REPO_ROOT}/docs/images"
rm -f "${REPO_ROOT}/docs/app.js" "${REPO_ROOT}/docs/styles.css"

cp -r "${REPO_ROOT}/apps/web/dist/"* "${REPO_ROOT}/docs/"

# 3. Ensure data/generated/schedule.json is synced
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/docs/schedule.json"
cp "${REPO_ROOT}/data/generated/schedule.json" "${REPO_ROOT}/docs/mock_schedule.json"

echo "=== Build and artifact sync completed successfully! ==="
