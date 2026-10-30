import test from 'node:test';
import assert from 'node:assert/strict';
import { getDeadlineInfo, parseDeadlineDate } from './date.ts';

test('parseDeadlineDate parses YYYY-MM-DD correctly', () => {
  const result = parseDeadlineDate('2026-11-22');
  assert.deepEqual(result, { year: 2026, month: 11, day: 22 });
});

test('parseDeadlineDate handles invalid strings gracefully', () => {
  assert.equal(parseDeadlineDate(''), null);
  assert.equal(parseDeadlineDate('invalid'), null);
});

test('getDeadlineInfo calculates daysLeft correctly for future date', () => {
  const mockNow = new Date(2026, 9, 30); // 2026-10-30
  const info = getDeadlineInfo('2026-11-22', mockNow);
  assert.ok(info);
  assert.equal(info.daysLeft, 23);
  assert.equal(info.formattedDeadline, '11/22(日)まで');
  assert.equal(info.countdownText, 'あと23日');
  assert.equal(info.isExpired, false);
  assert.equal(info.isUrgent, false);
});

test('getDeadlineInfo handles urgent deadline (3 days left)', () => {
  const mockNow = new Date(2026, 10, 19); // 2026-11-19
  const info = getDeadlineInfo('2026-11-22', mockNow);
  assert.ok(info);
  assert.equal(info.daysLeft, 3);
  assert.equal(info.countdownText, 'あと3日');
  assert.equal(info.isUrgent, true);
  assert.equal(info.isExpired, false);
});

test('getDeadlineInfo handles tomorrow deadline (1 day left)', () => {
  const mockNow = new Date(2026, 10, 21); // 2026-11-21
  const info = getDeadlineInfo('2026-11-22', mockNow);
  assert.ok(info);
  assert.equal(info.daysLeft, 1);
  assert.equal(info.countdownText, 'あと1日 (明日締切)');
  assert.equal(info.isUrgent, true);
  assert.equal(info.isExpired, false);
});

test('getDeadlineInfo handles same-day deadline', () => {
  const mockNow = new Date(2026, 10, 22); // 2026-11-22
  const info = getDeadlineInfo('2026-11-22', mockNow);
  assert.ok(info);
  assert.equal(info.daysLeft, 0);
  assert.equal(info.countdownText, '本日締切');
  assert.equal(info.isUrgent, true);
  assert.equal(info.isExpired, false);
});

test('getDeadlineInfo handles expired deadline', () => {
  const mockNow = new Date(2026, 10, 23); // 2026-11-23
  const info = getDeadlineInfo('2026-11-22', mockNow);
  assert.ok(info);
  assert.equal(info.daysLeft, -1);
  assert.equal(info.countdownText, '受付終了');
  assert.equal(info.isExpired, true);
});

test('getDeadlineInfo returns null when deadline is omitted', () => {
  assert.equal(getDeadlineInfo(undefined), null);
});
