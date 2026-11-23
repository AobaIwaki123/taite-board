#!/usr/bin/env bash
# next-version.sh: Derives the next semantic version from git tags and Conventional Commits
# without mutating tracked files in the repository (Anti-Git Churn).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

OVERRIDE_VERSION=""
GITHUB_OUTPUT_MODE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --override)
      OVERRIDE_VERSION="${2:-}"
      shift 2
      ;;
    --github-output)
      GITHUB_OUTPUT_MODE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--override <version>] [--github-output]"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

cd "${ROOT_DIR}"

emit_output() {
  local app_version="$1"
  local tag_name="$2"
  local needs_tag="$3"
  local bump_type="$4"
  local base_tag="$5"

  if [[ "${GITHUB_OUTPUT_MODE}" == "true" && -n "${GITHUB_OUTPUT:-}" ]]; then
    {
      echo "app_version=${app_version}"
      echo "tag_name=${tag_name}"
      echo "needs_tag=${needs_tag}"
      echo "bump_type=${bump_type}"
      echo "base_tag=${base_tag}"
    } >> "${GITHUB_OUTPUT}"
  fi

  echo "app_version=${app_version}"
  echo "tag_name=${tag_name}"
  echo "needs_tag=${needs_tag}"
  echo "bump_type=${bump_type}"
  echo "base_tag=${base_tag}"
}

if [[ -n "${OVERRIDE_VERSION}" ]]; then
  clean_ver="${OVERRIDE_VERSION#v}"
  tag_name="v${clean_ver}"
  needs_tag="true"
  if git rev-parse "refs/tags/${tag_name}" >/dev/null 2>&1; then
    needs_tag="false"
  fi
  emit_output "${clean_ver}" "${tag_name}" "${needs_tag}" "manual" "${tag_name}"
  exit 0
fi

# 1. Reuse existing semver tag if HEAD is already tagged
HEAD_TAG="$(git tag --points-at HEAD --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1 || true)"
if [[ -n "${HEAD_TAG}" ]]; then
  emit_output "${HEAD_TAG#v}" "${HEAD_TAG}" "false" "existing" "${HEAD_TAG}"
  exit 0
fi

# 2. Find latest semver tag (fallback to VERSION file or 0.0.0)
BASE_TAG="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1 || true)"
if [[ -z "${BASE_TAG}" ]]; then
  if [[ -f "${ROOT_DIR}/VERSION" ]]; then
    BASE_VER="$(tr -d '[:space:]' < "${ROOT_DIR}/VERSION")"
    BASE_TAG="v${BASE_VER#v}"
  else
    BASE_TAG="v0.0.0"
  fi
  LOG_RANGE="HEAD"
else
  LOG_RANGE="${BASE_TAG}..HEAD"
fi

BASE_CLEAN="${BASE_TAG#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "${BASE_CLEAN}"
MAJOR="${MAJOR:-0}"
MINOR="${MINOR:-0}"
PATCH="${PATCH:-0}"

# 3. Inspect commits since BASE_TAG using Conventional Commits
SUBJECTS="$(git log "${LOG_RANGE}" --format=%s 2>/dev/null || true)"
BODIES="$(git log "${LOG_RANGE}" --format=%b 2>/dev/null || true)"

BUMP_TYPE="patch"
if echo "${SUBJECTS}" | grep -qE '^[a-zA-Z0-9_-]+(\([^)]+\))?!:' || echo "${BODIES}" | grep -qE '^BREAKING CHANGE:'; then
  BUMP_TYPE="major"
  MAJOR=$((MAJOR + 1))
  MINOR=0
  PATCH=0
elif echo "${SUBJECTS}" | grep -qE '^feat(\([^)]+\))?:'; then
  BUMP_TYPE="minor"
  MINOR=$((MINOR + 1))
  PATCH=0
else
  BUMP_TYPE="patch"
  PATCH=$((PATCH + 1))
fi

NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
TAG_NAME="v${NEW_VERSION}"

# Guarantee tag uniqueness if an out-of-band tag already exists
while git rev-parse "refs/tags/${TAG_NAME}" >/dev/null 2>&1; do
  PATCH=$((PATCH + 1))
  NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
  TAG_NAME="v${NEW_VERSION}"
done

emit_output "${NEW_VERSION}" "${TAG_NAME}" "true" "${BUMP_TYPE}" "${BASE_TAG}"
