import fs from 'node:fs';
import path from 'node:path';

export interface TypographyViolation {
  filePath: string;
  lineNumber: number;
  lineText: string;
  matched: string;
  reason: string;
}

const MIN_PX = 12.0;
const MIN_REM = 0.75;
const MIN_EM = 0.75;
const MIN_PT = 9.0;

/**
 * Strips JS/TS and HTML comments while preserving exact line numbers and column offsets.
 */
export function stripComments(content: string): string {
  // Replace block comments /* ... */
  const noBlock = content.replace(/\/\*[\s\S]*?\*\//g, (match) => {
    return match.replace(/[^\n]/g, ' ');
  });
  // Replace single-line comments // ...
  const noLine = noBlock.replace(/\/\/[^\n]*/g, (match) => {
    return ' '.repeat(match.length);
  });
  // Replace HTML comments <!-- ... -->
  return noLine.replace(/<!--[\s\S]*?-->/g, (match) => {
    return match.replace(/[^\n]/g, ' ');
  });
}

/**
 * Checks a line of text for font size specifications strictly below the 12px Floor Standard.
 * Standard Tailwind classes (text-xs, text-sm, etc.) are >= 12px and pass.
 * Disallowed patterns include:
 * - text bracket with sub-12px size
 * - font-size below 12px
 * - fontSize property below 12px
 */
export function findTypographyViolationsInContent(
  content: string,
  filePath: string
): TypographyViolation[] {
  const violations: TypographyViolation[] = [];
  const sanitizedContent = stripComments(content);
  const lines = sanitizedContent.split('\n');
  const originalLines = content.split('\n');

  // Regex patterns
  const tailwindArbitraryRegex = /\btext-\[([0-9.]+)(px|rem|em|pt)\]/g;
  const cssFontSizeRegex = /font-size\s*:\s*([0-9.]+)(px|rem|em|pt)/gi;
  const inlineFontSizeRegex = /fontSize\s*:\s*['"]?([0-9.]+)(px|rem|em|pt)?['"]?/gi;

  lines.forEach((line, index) => {
    const lineNumber = index + 1;

    // 1. Tailwind arbitrary text size
    let match: RegExpExecArray | null;
    while ((match = tailwindArbitraryRegex.exec(line)) !== null) {
      const fullMatch = match[0];
      const value = parseFloat(match[1]);
      const unit = match[2].toLowerCase();

      if (isSubFloor(value, unit)) {
        violations.push({
          filePath,
          lineNumber,
          lineText: originalLines[index].trim(),
          matched: fullMatch,
          reason: `Found "${fullMatch}" (${value}${unit}) which is below the 12px Floor Standard (text-xs = 0.75rem / 13.1px).`,
        });
      }
    }

    // 2. CSS font-size
    while ((match = cssFontSizeRegex.exec(line)) !== null) {
      const fullMatch = match[0];
      const value = parseFloat(match[1]);
      const unit = match[2].toLowerCase();

      if (isSubFloor(value, unit)) {
        violations.push({
          filePath,
          lineNumber,
          lineText: originalLines[index].trim(),
          matched: fullMatch,
          reason: `Found CSS "${fullMatch}" which is below the 12px Floor Standard.`,
        });
      }
    }

    // 3. JSX inline fontSize
    while ((match = inlineFontSizeRegex.exec(line)) !== null) {
      const fullMatch = match[0];
      const value = parseFloat(match[1]);
      const unit = (match[2] || 'px').toLowerCase();

      if (isSubFloor(value, unit)) {
        violations.push({
          filePath,
          lineNumber,
          lineText: originalLines[index].trim(),
          matched: fullMatch,
          reason: `Found inline style "${fullMatch}" which is below the 12px Floor Standard.`,
        });
      }
    }
  });

  return violations;
}

function isSubFloor(value: number, unit: string): boolean {
  if (isNaN(value)) return false;
  switch (unit) {
    case 'px':
      return value < MIN_PX;
    case 'rem':
      return value < MIN_REM;
    case 'em':
      return value < MIN_EM;
    case 'pt':
      return value < MIN_PT;
    default:
      return false;
  }
}

/**
 * Scans a directory recursively for source files and returns all detected typography violations.
 */
export function scanDirectoryForTypographyViolations(
  dirPath: string,
  validExtensions: string[] = ['.tsx', '.ts', '.jsx', '.js', '.css', '.html'],
  ignoredPatterns: RegExp[] = [/\.test\.ts$/, /node_modules/, /dist/, /\.git/]
): TypographyViolation[] {
  const violations: TypographyViolation[] = [];

  function walk(currentPath: string) {
    if (!fs.existsSync(currentPath)) return;
    const stat = fs.statSync(currentPath);

    if (stat.isDirectory()) {
      const entries = fs.readdirSync(currentPath);
      for (const entry of entries) {
        const fullPath = path.join(currentPath, entry);
        if (ignoredPatterns.some((pattern) => pattern.test(fullPath))) {
          continue;
        }
        walk(fullPath);
      }
    } else if (stat.isFile()) {
      const ext = path.extname(currentPath);
      if (
        validExtensions.includes(ext) &&
        !ignoredPatterns.some((pattern) => pattern.test(currentPath))
      ) {
        const content = fs.readFileSync(currentPath, 'utf-8');
        const fileViolations = findTypographyViolationsInContent(content, currentPath);
        violations.push(...fileViolations);
      }
    }
  }

  walk(dirPath);
  return violations;
}

export function formatTypographyViolations(violations: TypographyViolation[]): string {
  return violations
    .map(
      (v) =>
        `  - ${v.filePath}:${v.lineNumber}\n    ${v.reason}\n    Source line: ${v.lineText}`
    )
    .join('\n');
}
