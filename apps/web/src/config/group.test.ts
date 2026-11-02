import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveCurrentGroup, REGISTERED_GROUPS, DEFAULT_GROUP_ID, isPortalMode } from './group.ts';

test('resolveCurrentGroup returns default group when window is undefined', () => {
  const group = resolveCurrentGroup();
  assert.equal(group.id, DEFAULT_GROUP_ID);
  assert.equal(group.name, 'リボンクロック');
  assert.equal(group.dataUrl, 'schedule.json');
});

test('REGISTERED_GROUPS contains valid default and template configurations', () => {
  assert.ok(REGISTERED_GROUPS.rincent);
  assert.equal(REGISTERED_GROUPS.rincent.storagePrefix, 'ribonclock_');
  assert.equal(
    REGISTERED_GROUPS.rincent.heroImage,
    'https://pbs.twimg.com/media/HVi3dC9bsAAinY6?format=jpg&name=large'
  );
  assert.ok(REGISTERED_GROUPS.dollykiss);
  assert.equal(REGISTERED_GROUPS.dollykiss.storagePrefix, 'dollykiss_');
  assert.equal(REGISTERED_GROUPS.dollykiss.name, 'Dolly Kiss');
  assert.equal(
    REGISTERED_GROUPS.dollykiss.heroImage,
    'https://pbs.twimg.com/media/HVumF5haAAAHcXU.jpg?name=large'
  );
  assert.ok(REGISTERED_GROUPS.sample);
  assert.equal(REGISTERED_GROUPS.sample.theme.primary, 'emerald');
});

test('isPortalMode identifies portal view routing conditions accurately', () => {
  // 1. Root domain taite.iorin.net
  assert.equal(
    isPortalMode({ hostname: 'taite.iorin.net', search: '' }),
    true
  );

  // 2. Query param ?view=portal overrides subdomain
  assert.equal(
    isPortalMode({ hostname: 'rincent.iorin.net', search: '?view=portal' }),
    true
  );

  // 3. Localhost without ?group= query param defaults to portal
  assert.equal(
    isPortalMode({ hostname: 'localhost', search: '' }),
    true
  );

  // 4. Localhost with ?group= opens group timetable
  assert.equal(
    isPortalMode({ hostname: 'localhost', search: '?group=rincent' }),
    false
  );

  // 5. Group subdomain without portal override opens group timetable
  assert.equal(
    isPortalMode({ hostname: 'rincent.iorin.net', search: '' }),
    false
  );
  assert.equal(
    isPortalMode({ hostname: 'dollykiss.iorin.net', search: '' }),
    false
  );
});
