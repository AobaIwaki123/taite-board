#!/usr/bin/env bash
# ship.sh: End-to-end mechanical shipping pipeline.
# Runs verify, commits, pushes, creates PR, squash-merges, and updates local main.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

BRANCH=""
TITLE=""
BODY=""

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch|-b)
      BRANCH="$2"
      shift 2
      ;;
    *)
      if [[ -z "${TITLE}" ]]; then
        TITLE="$1"
      elif [[ -z "${BODY}" ]]; then
        BODY="$1"
      else
        BODY="${BODY}"$'\n\n'"$1"
      fi
      shift
      ;;
  esac
done

if [[ -z "${TITLE}" ]]; then
  echo "Usage: $0 [--branch <branch-name>] \"<title>\" [\"<body>\"]" >&2
  exit 1
fi

if [[ -z "${BODY}" ]]; then
  BODY="## Summary"$'\n'"- ${TITLE}"$'\n\n'"## Verification"$'\n'"- Verified via ./scripts/verify.sh"
fi

cd "${ROOT_DIR}"

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# If currently on main, branch out
if [[ "${CURRENT_BRANCH}" == "main" ]]; then
  if [[ -z "${BRANCH}" ]]; then
    # Generate slug from title
    SLUG=$(echo "${TITLE}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g' | sed -E 's/^-+|-+$//g' | cut -c 1-40)
    BRANCH="feat/${SLUG}"
  fi
  echo "Creating and switching to branch: ${BRANCH}"
  git checkout -b "${BRANCH}"
else
  BRANCH="${CURRENT_BRANCH}"
  echo "Operating on current branch: ${BRANCH}"
fi

echo "=== Step 1: Running mechanical verification ==="
"${SCRIPT_DIR}/verify.sh"

echo "=== Step 2: Committing changes ==="
git add -A

if git diff --staged --quiet; then
  echo "No changes staged. Checking if remote branch needs PR..."
else
  git commit -m "${TITLE}"
fi

echo "=== Step 3: Pushing to origin ==="
git push -u origin "${BRANCH}"

echo "=== Step 4: Creating Pull Request ==="
PR_URL=$(gh pr create --title "${TITLE}" --body "${BODY}" || gh pr view --json url -q .url)
echo "Pull Request: ${PR_URL}"

PR_NUM=$(gh pr view "${BRANCH}" --json number -q .number)

echo "=== Step 5: Squash-merging PR #${PR_NUM} ==="
gh pr merge "${PR_NUM}" --squash --delete-branch

echo "=== Step 6: Updating local main ==="
git checkout main
git pull origin main

echo ""
echo "Successfully shipped PR #${PR_NUM} to main."
