import test from 'node:test';
import assert from 'node:assert/strict';
import {
  resolveCurrentGroup,
  REGISTERED_GROUPS,
  DEFAULT_GROUP_ID,
  GROUP_ALIASES,
  resolveGroupId,
  isPortalMode,
} from './group.ts';

test('resolveCurrentGroup returns default group when window is undefined', () => {
  const group = resolveCurrentGroup();
  assert.equal(group.id, DEFAULT_GROUP_ID);
  assert.equal(group.id, 'ribonclock');
  assert.equal(group.name, 'リボンクロック');
  assert.equal(group.shortName, 'リボクロ');
  assert.equal(group.dataUrl, 'data/groups/ribonclock/schedule.json');
});

test('REGISTERED_GROUPS contains valid default and template configurations', () => {
  assert.ok(REGISTERED_GROUPS.ribonclock);
  assert.equal(REGISTERED_GROUPS.ribonclock.id, 'ribonclock');
  assert.equal(REGISTERED_GROUPS.ribonclock.name, 'リボンクロック');
  assert.equal(REGISTERED_GROUPS.ribonclock.shortName, 'リボクロ');
  assert.equal(REGISTERED_GROUPS.ribonclock.storagePrefix, 'ribonclock_');
  assert.equal(
    REGISTERED_GROUPS.ribonclock.heroImage,
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

test('backward compatibility alias resolves rincent to ribonclock', () => {
  assert.equal(GROUP_ALIASES.rincent, 'ribonclock');
  assert.equal(resolveGroupId('rincent'), 'ribonclock');
  assert.equal(resolveGroupId('ribonclock'), 'ribonclock');
  assert.equal(resolveGroupId('dollykiss'), 'dollykiss');
  assert.equal(resolveGroupId('unknown_group'), undefined);
});

test('isPortalMode identifies portal view routing conditions accurately', () => {
  // 1. Root domain taite.iorin.net
  assert.equal(
    isPortalMode({ hostname: 'taite.iorin.net', search: '' }),
    true
  );

  // 2. Query param ?view=portal overrides subdomain
  assert.equal(
    isPortalMode({ hostname: 'ribonclock.iorin.net', search: '?view=portal' }),
    true
  );
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
    isPortalMode({ hostname: 'localhost', search: '?group=ribonclock' }),
    false
  );
  assert.equal(
    isPortalMode({ hostname: 'localhost', search: '?group=rincent' }),
    false
  );

  // 5. Group subdomain without portal override opens group timetable
  assert.equal(
    isPortalMode({ hostname: 'ribonclock.iorin.net', search: '' }),
    false
  );
  assert.equal(
    isPortalMode({ hostname: 'rincent.iorin.net', search: '' }),
    false
  );
  assert.equal(
    isPortalMode({ hostname: 'dollykiss.iorin.net', search: '' }),
    false
  );
});
