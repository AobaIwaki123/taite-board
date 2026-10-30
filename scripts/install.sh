#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=== Installing taite CLI ==="
cd "${REPO_ROOT}"
go install ./cmd/taite

GOBIN="$(go env GOBIN)"
if [ -z "${GOBIN}" ]; then
  GOBIN="$(go env GOPATH)/bin"
fi

echo "taite binary installed to: ${GOBIN}/taite"
if ! command -v taite >/dev/null 2>&1; then
  echo "Note: ${GOBIN} is not in your PATH. Add it via: export PATH=\"\${PATH}:${GOBIN}\""
fi
