#!/usr/bin/env bash
# verify-typography.sh: Mechanically verifies that no font size in source code
# or compiled CSS bundles falls below the 12px Floor Standard (text-xs / 0.75rem).
# Ref: note/07_mobile-typography-and-outdoor-readability.md & ADR-0017.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WEB_DIR="${ROOT_DIR}/apps/web"

echo "Checking typography floor standard (>= 12px / text-xs)..."

# 1. Run unit test suite (Tier 1 source scan + public assets scan)
(cd "${WEB_DIR}" && node --experimental-strip-types --test src/utils/typography.test.ts)

# 2. Inspect compiled production CSS bundle (Tier 2 compiled artifact scan)
DIST_ASSETS_DIR="${WEB_DIR}/dist/assets"
if [ -d "${DIST_ASSETS_DIR}" ]; then
  VIOLATIONS_COUNT=0
  for css_file in "${DIST_ASSETS_DIR}"/*.css; do
    [ -f "${css_file}" ] || continue
    # Node inline inspector to parse font-size properties in minified CSS
    VIOLATIONS=$(node -e '
      const fs = require("fs");
      const content = fs.readFileSync(process.argv[1], "utf-8");
      const matches = [...content.matchAll(/font-size\s*:\s*([0-9.]+)(px|rem|em|pt)/gi)];
      const violations = [];
      for (const m of matches) {
        const val = parseFloat(m[1]);
        const unit = m[2].toLowerCase();
        let isBad = false;
        if (unit === "px" && val < 12.0) isBad = true;
        else if (unit === "rem" && val < 0.75) isBad = true;
        else if (unit === "em" && val < 0.75) isBad = true;
        else if (unit === "pt" && val < 9.0) isBad = true;

        if (isBad) {
          violations.push(`${m[0]} (${val}${unit})`);
        }
      }
      if (violations.length > 0) {
        console.error(violations.join(", "));
        process.exit(1);
      }
    ' "${css_file}" 2>&1 || true)

    if [ -n "${VIOLATIONS}" ]; then
      echo "ERROR: Sub-12px font-size detected in compiled CSS bundle: ${css_file}" >&2
      echo "  Violating rules: ${VIOLATIONS}" >&2
      echo "  Minimum allowed size is 12px (text-xs = 0.75rem / 13.1px)." >&2
      echo "  See note/07_mobile-typography-and-outdoor-readability.md" >&2
      exit 1
    fi
  done
  echo "✓ Production CSS bundle inspected: zero sub-12px rules found."
fi

echo "✓ Typography verification PASSED (Floor standard >= 12px enforced)."
