import test from 'node:test';
import assert from 'node:assert/strict';
import {
  getDeadlineInfo,
  parseDeadlineDate,
  getFreshnessInfo,
  isSessionPast,
  resolveNextSession,
} from './date.ts';

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

test('getDeadlineInfo supports event suffix mode (開催)', () => {
  const mockNow = new Date(2026, 9, 30); // 2026-10-30
  const info = getDeadlineInfo('2026-12-06', mockNow, { suffix: '開催' });
  assert.ok(info);
  assert.equal(info.daysLeft, 37);
  assert.equal(info.formattedDeadline, '12/6(日)開催');
  assert.equal(info.countdownText, 'あと37日');
  assert.equal(info.isUrgent, false);
  assert.equal(info.isExpired, false);
});

test('getDeadlineInfo handles event day (本日開催！)', () => {
  const mockNow = new Date(2026, 11, 6); // 2026-12-06
  const info = getDeadlineInfo('2026-12-06', mockNow, { suffix: '開催' });
  assert.ok(info);
  assert.equal(info.daysLeft, 0);
  assert.equal(info.countdownText, '本日開催！');
  assert.equal(info.isUrgent, true);
});

test('getDeadlineInfo returns null when deadline is omitted', () => {
  assert.equal(getDeadlineInfo(undefined), null);
});

// --- Freshness Tests (ADR-0008) ---

test('getFreshnessInfo returns null when online (not fallback) regardless of age', () => {
  const mockNow = new Date('2026-10-31T20:00:00Z');
  // Even if generated 48 hours ago, online users should not see stale warnings
  const info = getFreshnessInfo({
    isFallback: false,
    generatedAt: '2026-10-29T20:00:00Z',
    nowDate: mockNow,
  });
  assert.equal(info, null);
});

test('getFreshnessInfo returns recent offline message when under 24 hours in fallback', () => {
  const mockNow = new Date('2026-10-31T20:00:00Z');
  const info = getFreshnessInfo({
    isFallback: true,
    generatedAt: '2026-10-31T05:00:00Z', // 15 hours ago
    nowDate: mockNow,
  });
  assert.deepEqual(info, {
    label: 'オフライン保存版を表示中',
    isStale: false,
  });
});

test('getFreshnessInfo returns days ago offline message when 24+ hours in fallback', () => {
  const mockNow = new Date('2026-10-31T20:00:00Z');
  const info = getFreshnessInfo({
    isFallback: true,
    generatedAt: '2026-10-30T08:00:00Z', // 36 hours ago (1 day)
    nowDate: mockNow,
  });
  assert.deepEqual(info, {
    label: 'オフライン保存版（1日前）',
    isStale: true,
  });

  const info2Days = getFreshnessInfo({
    isFallback: true,
    generatedAt: '2026-10-29T08:00:00Z', // 60 hours ago (2 days)
    nowDate: mockNow,
  });
  assert.deepEqual(info2Days, {
    label: 'オフライン保存版（2日前）',
    isStale: true,
  });
});

test('getFreshnessInfo handles fallback with missing or invalid date', () => {
  assert.deepEqual(getFreshnessInfo({ isFallback: true }), {
    label: 'オフライン保存版を表示中',
    isStale: false,
  });
  assert.deepEqual(getFreshnessInfo({ isFallback: true, generatedAt: 'invalid-date' }), {
    label: 'オフライン保存版を表示中',
    isStale: false,
  });
});

// --- Session Past & Next Session Resolver Tests ---

test('isSessionPast detects past, upcoming, and post-benefit grace period correctly', () => {
  const session = {
    date: '2026-10-31',
    live: { start_time: '12:50', end_time: '13:15' },
    benefit: { start_time: '13:20', end_time: '14:20' },
  };

  // 1. Previous day
  const beforeDay = new Date(2026, 9, 30, 12, 0); // 10/30 12:00
  assert.equal(isSessionPast(session, beforeDay), false);

  // 2. Next day
  const nextDay = new Date(2026, 10, 1, 9, 0); // 11/01 09:00
  assert.equal(isSessionPast(session, nextDay), true);

  // 3. Same day before benefit ends
  const duringBenefit = new Date(2026, 9, 31, 13, 50); // 10/31 13:50
  assert.equal(isSessionPast(session, duringBenefit), false);

  // 4. Same day 1 hour after benefit ends (within 2h grace period)
  const oneHourAfter = new Date(2026, 9, 31, 15, 20); // 10/31 15:20
  assert.equal(isSessionPast(session, oneHourAfter, 2), false);

  // 5. Same day 2.5 hours after benefit ends (past 2h grace period)
  const twoAndHalfHoursAfter = new Date(2026, 9, 31, 16, 51); // 10/31 16:51
  assert.equal(isSessionPast(session, twoAndHalfHoursAfter, 2), true);
});

test('resolveNextSession advances to upcoming session after past session ends', () => {
  const sessions = [
    {
      id: '20261031-1',
      date: '2026-10-31',
      live: { start_time: '12:50', end_time: '13:15' },
      benefit: { start_time: '13:20', end_time: '14:20' },
    },
    {
      id: '20261101-1',
      date: '2026-11-01',
      live: { start_time: '15:05', end_time: '15:30' },
      benefit: { start_time: '15:35', end_time: '16:35' },
    },
    {
      id: '20261101-2',
      date: '2026-11-01',
      live: { start_time: '18:35', end_time: '18:55' },
      benefit: { start_time: '19:05', end_time: '20:05' },
    },
  ];

  // While 10/31 event is fresh / upcoming
  const duringEvent = new Date(2026, 9, 31, 14, 0);
  assert.equal(resolveNextSession(sessions, '20261031-1', duringEvent)?.id, '20261031-1');

  // Hours after 10/31 event ends (e.g. 10/31 18:00)
  const eveningOf1031 = new Date(2026, 9, 31, 18, 0);
  assert.equal(resolveNextSession(sessions, '20261031-1', eveningOf1031)?.id, '20261101-1');

  // Next morning (11/01 10:00)
  const morningOf1101 = new Date(2026, 10, 1, 10, 0);
  assert.equal(resolveNextSession(sessions, '20261031-1', morningOf1101)?.id, '20261101-1');

  // After 11/01-1 ends + 2h grace period (16:35 + 2h = 18:35 -> test at 18:40), advances to 11/01-2
  const eveningOf1101 = new Date(2026, 10, 1, 18, 40);
  assert.equal(resolveNextSession(sessions, '20261031-1', eveningOf1101)?.id, '20261101-2');
});
