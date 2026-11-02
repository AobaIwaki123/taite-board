import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveCurrentGroup, REGISTERED_GROUPS, DEFAULT_GROUP_ID } from './group.ts';

test('resolveCurrentGroup returns default group when window is undefined', () => {
  const group = resolveCurrentGroup();
  assert.equal(group.id, DEFAULT_GROUP_ID);
  assert.equal(group.name, 'リボンクロック');
  assert.equal(group.dataUrl, 'schedule.json');
});

test('REGISTERED_GROUPS contains valid default and template configurations', () => {
  assert.ok(REGISTERED_GROUPS.rincent);
  assert.equal(REGISTERED_GROUPS.rincent.storagePrefix, 'ribonclock_');
  assert.ok(REGISTERED_GROUPS.sample);
  assert.equal(REGISTERED_GROUPS.sample.theme.primary, 'emerald');
});
