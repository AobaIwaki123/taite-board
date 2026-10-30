#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=== Building Web Frontend (apps/web) ==="
cd "${REPO_ROOT}/apps/web"
if [ ! -d "node_modules" ]; then
  echo "Installing npm dependencies..."
  npm ci
fi
npm run build

echo "=== Build completed successfully! ==="
