import test from 'node:test';
import assert from 'node:assert/strict';
import { parseSpecialNotes } from './notes.ts';

test('parseSpecialNotes returns empty array for empty or null input', () => {
  assert.deepEqual(parseSpecialNotes(undefined), []);
  assert.deepEqual(parseSpecialNotes(null), []);
  assert.deepEqual(parseSpecialNotes('   '), []);
});

test('parseSpecialNotes splits slash-joined notes and deduplicates entries', () => {
  assert.deepEqual(
    parseSpecialNotes('私服公演 / 声出しOK / 私服公演\n整理券は17:30配布'),
    ['私服公演', '声出しOK', '整理券は17:30配布'],
  );
});

test('parseSpecialNotes preserves slashes without spaces inside dates or phrases', () => {
  assert.deepEqual(
    parseSpecialNotes('11/28発売記念 / 前物販あり'),
    ['11/28発売記念', '前物販あり'],
  );
});
