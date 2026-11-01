import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  findTypographyViolationsInContent,
  scanDirectoryForTypographyViolations,
  formatTypographyViolations,
} from './typography.ts';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const webSrcDir = path.resolve(__dirname, '..');
const webPublicDir = path.resolve(__dirname, '../../public');

test('findTypographyViolationsInContent detects sub-12px Tailwind arbitrary sizes', () => {
  // Construct strings dynamically so Tailwind v4 scanner does not extract them into the production CSS bundle
  const prefix = 'text';
  const t10 = `${prefix}-[10px]`;
  const t11 = `${prefix}-[11px]`;
  const t065 = `${prefix}-[0.65rem]`;
  const code = [
    'export function BadComponent() {',
    '  return (',
    '    <div class="' + t10 + ' text-stone-400">',
    '      <span class="' + t11 + '">Warning</span>',
    '      <span class="' + t065 + '">Small Rem</span>',
    '    </div>',
    '  );',
    '}',
  ].join('\n');

  const violations = findTypographyViolationsInContent(code, 'BadComponent.tsx');
  assert.equal(violations.length, 3);
  assert.equal(violations[0].matched, t10);
  assert.equal(violations[1].matched, t11);
  assert.equal(violations[2].matched, t065);
});

test('findTypographyViolationsInContent allows compliant font sizes (>= 12px or >= 0.75rem)', () => {
  const prefix = 'text';
  const t12 = `${prefix}-[12px]`;
  const t13 = `${prefix}-[13px]`;
  const t075 = `${prefix}-[0.75rem]`;
  const t1rem = `${prefix}-[1rem]`;
  const code = [
    'export function GoodComponent() {',
    '  return (',
    '    <div class="text-xs text-sm text-base text-lg text-xl text-3xl">',
    '      <span class="' + t12 + '">Floor Allowed</span>',
    '      <span class="' + t13 + '">Above Floor</span>',
    '      <span class="' + t075 + '">0.75rem Allowed</span>',
    '      <span class="' + t1rem + '">1rem Allowed</span>',
    '      <span class="text-[#f43f5e]">Arbitrary Color Not Size</span>',
    '      <span class="text-[var(--custom)]">Var Color</span>',
    '    </div>',
    '  );',
    '}',
  ].join('\n');

  const violations = findTypographyViolationsInContent(code, 'GoodComponent.tsx');
  assert.equal(violations.length, 0);
});

test('findTypographyViolationsInContent detects CSS and inline font-size violations', () => {
  const code = `
    .bad-css {
      font-size: 10px;
    }
    const style = { fontSize: '11px' };
    const numStyle = { fontSize: 9 };
  `;
  const violations = findTypographyViolationsInContent(code, 'styles.css');
  assert.equal(violations.length, 3);
});

test('Production codebase (apps/web/src) strictly obeys the 12px Floor Standard', () => {
  const srcViolations = scanDirectoryForTypographyViolations(webSrcDir);
  if (srcViolations.length > 0) {
    const errorDetails = formatTypographyViolations(srcViolations);
    assert.fail(
      `Found ${srcViolations.length} typography floor standard violation(s) in apps/web/src:\n${errorDetails}\n\nMinimum allowed font size is 12px (Tailwind text-xs = 0.75rem / 13.1px). See note/07_mobile-typography-and-outdoor-readability.md.`
    );
  }
  assert.equal(srcViolations.length, 0);
});

test('Production public assets (apps/web/public) strictly obey the 12px Floor Standard', () => {
  const publicViolations = scanDirectoryForTypographyViolations(webPublicDir);
  if (publicViolations.length > 0) {
    const errorDetails = formatTypographyViolations(publicViolations);
    assert.fail(
      `Found ${publicViolations.length} typography floor standard violation(s) in apps/web/public:\n${errorDetails}\n\nMinimum allowed font size is 12px (Tailwind text-xs = 0.75rem / 13.1px). See note/07_mobile-typography-and-outdoor-readability.md.`
    );
  }
  assert.equal(publicViolations.length, 0);
});
