#!/usr/bin/env bash
# release.sh: Lightweight semantic versioning and release tag automation for taite-board.
# Usage: ./scripts/release.sh [patch|minor|major|<version>] [--dry-run]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION_FILE="${ROOT_DIR}/VERSION"
PKG_FILE="${ROOT_DIR}/apps/web/package.json"

DRY_RUN=false
TARGET_TYPE="patch"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    patch|minor|major)
      TARGET_TYPE="$1"
      shift
      ;;
    v[0-9]*|[0-9]*)
      TARGET_TYPE="${1#v}" # strip leading v
      shift
      ;;
    -h|--help)
      echo "Usage: ./scripts/release.sh [patch|minor|major|<version>] [--dry-run]"
      echo ""
      echo "Increments SemVer, runs verify.sh, commits, tags vX.Y.Z, and creates a GitHub Release."
      exit 0
      ;;
    *)
      echo "Unknown option or argument: $1" >&2
      exit 1
      ;;
  esac
done

# Read current version
if [[ -f "${VERSION_FILE}" ]]; then
  CURRENT_VERSION="$(tr -d '[:space:]' < "${VERSION_FILE}")"
elif [[ -f "${PKG_FILE}" ]]; then
  CURRENT_VERSION="$(node -p "require('${PKG_FILE}').version")"
else
  CURRENT_VERSION="1.0.0"
fi

# Calculate new version
IFS='.' read -r MAJOR MINOR PATCH <<< "${CURRENT_VERSION}"
MAJOR="${MAJOR:-1}"
MINOR="${MINOR:-0}"
PATCH="${PATCH:-0}"

case "${TARGET_TYPE}" in
  patch)
    NEW_VERSION="${MAJOR}.${MINOR}.$((PATCH + 1))"
    ;;
  minor)
    NEW_VERSION="${MAJOR}.$((MINOR + 1)).0"
    ;;
  major)
    NEW_VERSION="$((MAJOR + 1)).0.0"
    ;;
  *)
    NEW_VERSION="${TARGET_TYPE}"
    ;;
esac

TAG_NAME="v${NEW_VERSION}"

echo "=================================================="
echo "Ribbon Clock Release Tool"
echo "=================================================="
echo "Current version: v${CURRENT_VERSION}"
echo "Target version:  ${TAG_NAME}"
echo "Dry-run mode:    ${DRY_RUN}"
echo "=================================================="

# Check if tag already exists
if git rev-parse "${TAG_NAME}" >/dev/null 2>&1; then
  echo "Error: Git tag ${TAG_NAME} already exists!" >&2
  exit 1
fi

# Find previous tag for changelog preview
PREV_TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo "")"
echo "Recent commits since ${PREV_TAG:-repository creation}:"
if [[ -n "${PREV_TAG}" ]]; then
  git log "${PREV_TAG}..HEAD" --oneline || true
else
  git log -n 10 --oneline || true
fi
echo ""

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY-RUN] Version calculation and validation successful."
  echo "[DRY-RUN] Would update: VERSION, apps/web/package.json"
  echo "[DRY-RUN] Would create commit: chore(release): bump version to ${TAG_NAME}"
  echo "[DRY-RUN] Would create tag: ${TAG_NAME}"
  echo "[DRY-RUN] Would push to origin and trigger GitHub Release."
  exit 0
fi

# Ensure clean working tree
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: Working directory has uncommitted changes. Please commit or stash first." >&2
  exit 1
fi

echo "=== [1/5] Updating version files ==="
echo "${NEW_VERSION}" > "${VERSION_FILE}"

if [[ -f "${PKG_FILE}" ]]; then
  npm --prefix "${ROOT_DIR}/apps/web" version "${NEW_VERSION}" --no-git-tag-version --allow-same-version >/dev/null
fi
echo "Updated ${VERSION_FILE} and ${PKG_FILE} to ${NEW_VERSION}"

echo "=== [2/5] Running mechanical verification pipeline ==="
if ! "${SCRIPT_DIR}/verify.sh"; then
  echo "Verification failed! Reverting version files..." >&2
  git checkout "${VERSION_FILE}" "${PKG_FILE}"
  exit 1
fi

echo "=== [3/5] Committing version bump ==="
git add "${VERSION_FILE}" "${PKG_FILE}" "${ROOT_DIR}/apps/web/package-lock.json" 2>/dev/null || true
git commit -m "chore(release): bump version to ${TAG_NAME}"

echo "=== [4/5] Creating annotated git tag ==="
git tag -a "${TAG_NAME}" -m "Release ${TAG_NAME}"

echo "=== [5/5] Pushing changes and tag to origin ==="
git push origin HEAD:main
git push origin "${TAG_NAME}"

if command -v gh >/dev/null 2>&1; then
  echo "Creating GitHub Release via gh CLI..."
  gh release create "${TAG_NAME}" --title "Release ${TAG_NAME}" --generate-notes || true
fi

echo ""
echo "Successfully released ${TAG_NAME}!"
