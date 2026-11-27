/**
 * Splits concatenated special_notes (" / " or newline separated) into unique,
 * trimmed display lines while preserving original order.
 */
export function parseSpecialNotes(raw?: string | null): string[] {
  if (!raw) return [];
  const trimmed = raw.trim();
  if (!trimmed) return [];

  const parts = trimmed
    .split(/\r?\n|\s+\/\s+/)
    .map((p) => p.trim())
    .filter(Boolean);

  const seen = new Set<string>();
  const result: string[] = [];
  for (const part of parts) {
    if (seen.has(part)) continue;
    seen.add(part);
    result.push(part);
  }
  return result;
}
