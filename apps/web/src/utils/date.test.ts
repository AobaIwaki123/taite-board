import test from 'node:test';
import assert from 'node:assert/strict';
import {
  getDeadlineInfo,
  parseDeadlineDate,
  getFreshnessInfo,
  getSessionProgress,
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

test('isSessionPast detects past, upcoming, and ends immediately after benefit ends', () => {
  const session = {
    date: '2026-10-31',
    live: { start_time: '12:50', end_time: '13:15' },
    benefit: { start_time: '13:20', end_time: '14:20' },
  };

  // 1. Previous day -> not past
  const beforeDay = new Date(2026, 9, 30, 12, 0); // 10/30 12:00
  assert.equal(isSessionPast(session, beforeDay), false);

  // 2. Next day -> past
  const nextDay = new Date(2026, 10, 1, 9, 0); // 11/01 09:00
  assert.equal(isSessionPast(session, nextDay), true);

  // 3. Same day before benefit ends -> not past
  const duringBenefit = new Date(2026, 9, 31, 14, 0); // 10/31 14:00
  assert.equal(isSessionPast(session, duringBenefit), false);

  // 4. Same day 1 minute after benefit ends (default graceHours = 0) -> past!
  const justAfterBenefit = new Date(2026, 9, 31, 14, 21); // 10/31 14:21
  assert.equal(isSessionPast(session, justAfterBenefit), true);

  // 5. With delay offset of 10 min: 14:25 becomes effective 14:15 (inside benefit 13:20-14:20) -> not past!
  const at1425 = new Date(2026, 9, 31, 14, 25);
  assert.equal(isSessionPast(session, at1425, 0, 10), false);
});

test('resolveNextSession automatically advances immediately when a session ends', () => {
  const sessions = [
    {
      id: '20261101-1',
      date: '2026-11-01',
      live: { start_time: '12:00', end_time: '12:30' },
      benefit: { start_time: '12:40', end_time: '13:40' },
    },
    {
      id: '20261101-2',
      date: '2026-11-01',
      live: { start_time: '17:00', end_time: '17:30' },
      benefit: { start_time: '17:40', end_time: '18:40' },
    },
    {
      id: '20261103-1',
      date: '2026-11-03',
      live: { start_time: '18:30', end_time: '19:00' },
      benefit: { start_time: '19:15', end_time: '20:15' },
    },
  ];

  // 1. Morning before Part 1 (11:00) -> returns Part 1
  const morning = new Date(2026, 10, 1, 11, 0);
  assert.equal(resolveNextSession(sessions, '20261101-1', morning)?.id, '20261101-1');

  // 2. Part 1 benefit finishes at 13:40. At 13:45, Part 1 is finished -> immediately advances to Part 2!
  const afterPart1 = new Date(2026, 10, 1, 13, 45);
  assert.equal(resolveNextSession(sessions, '20261101-1', afterPart1)?.id, '20261101-2');

  // 3. Part 2 benefit finishes at 18:40. At 18:45, today is finished -> immediately advances to 11/03!
  const afterPart2 = new Date(2026, 10, 1, 18, 45);
  assert.equal(resolveNextSession(sessions, '20261101-1', afterPart2)?.id, '20261103-1');

  // 4. If Part 2 had a +15m delay: at 18:45, effective time is 18:30 (inside benefit) -> remains Part 2!
  const delayMap = { '20261101-2': 15 };
  assert.equal(resolveNextSession(sessions, '20261101-1', afterPart2, 0, delayMap)?.id, '20261101-2');
});

// --- Detailed Minute Countdown & Progress Tests ---

test('getSessionProgress provides accurate minute countdowns across all phases', () => {
  const date = '2026-11-01';
  const liveStart = '18:30';
  const liveEnd = '18:55';
  const benefitStart = '19:05';
  const benefitEnd = '20:05';

  // 1. Pre-live: 17:48 (42 minutes before 18:30 live)
  const at1748 = new Date(2026, 10, 1, 17, 48);
  const progPreLive = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1748, 0);
  assert.equal(progPreLive.status, 'upcoming');
  assert.equal(progPreLive.phase, 'pre_live');
  assert.equal(progPreLive.minutesRemaining, 42);
  assert.equal(progPreLive.countdownMessage, '出番開始まで あと42分');

  // 2. Pre-live: 16:00 (2 hours 30 minutes before 18:30 live)
  const at1600 = new Date(2026, 10, 1, 16, 0);
  const progHours = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1600, 0);
  assert.equal(progHours.countdownMessage, '出番開始まで あと2時間30分');

  // 3. Live in progress: 18:40 (15 minutes before 18:55 live end)
  const at1840 = new Date(2026, 10, 1, 18, 40);
  const progLive = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1840, 0);
  assert.equal(progLive.status, 'live');
  assert.equal(progLive.phase, 'live');
  assert.equal(progLive.minutesRemaining, 15);
  assert.equal(progLive.countdownMessage, '出番終了まで あと15分');

  // 4. Intermission / transition: 18:58 (7 minutes before 19:05 benefit start)
  const at1858 = new Date(2026, 10, 1, 18, 58);
  const progIntermission = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1858, 0);
  assert.equal(progIntermission.status, 'upcoming');
  assert.equal(progIntermission.phase, 'intermission');
  assert.equal(progIntermission.minutesRemaining, 7);
  assert.equal(progIntermission.countdownMessage, '特典会開始まで あと7分');

  // 5. Benefit in progress: 19:35 (30 minutes before 20:05 benefit end)
  const at1935 = new Date(2026, 10, 1, 19, 35);
  const progBenefit = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1935, 0);
  assert.equal(progBenefit.status, 'benefit');
  assert.equal(progBenefit.phase, 'benefit');
  assert.equal(progBenefit.minutesRemaining, 30);
  assert.equal(progBenefit.countdownMessage, '特典会終了まで あと30分');

  // 6. Ended: 20:10 (after benefit ends)
  const at2010 = new Date(2026, 10, 1, 20, 10);
  const progEnded = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at2010, 0);
  assert.equal(progEnded.status, 'ended');
  assert.equal(progEnded.phase, 'ended');
  assert.equal(progEnded.countdownMessage, '本日の全行程終了');
});

// --- Delay Offset Tests ---

test('getSessionProgress reflects schedule delay offset accurately', () => {
  const date = '2026-11-01';
  const liveStart = '18:30';
  const liveEnd = '18:55';
  const benefitStart = '19:05';
  const benefitEnd = '20:05';

  // 18:40 without delay -> Live in progress
  const at1840 = new Date(2026, 10, 1, 18, 40);
  const progNoDelay = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1840, 0);
  assert.equal(progNoDelay.status, 'live');
  assert.equal(progNoDelay.label, 'ライブ中');

  // 19:00 without delay -> Between live and benefit (or upcoming benefit)
  const at1900 = new Date(2026, 10, 1, 19, 0);
  const progAt1900 = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1900, 0);
  assert.equal(progAt1900.status, 'upcoming');

  // 19:00 with 15min delay -> 19:00 - 15m = 18:45, which is inside live (18:30~18:55)!
  const progWithDelay = getSessionProgress(date, liveStart, liveEnd, benefitStart, benefitEnd, at1900, 15);
  assert.equal(progWithDelay.status, 'live');
  assert.equal(progWithDelay.label, 'ライブ中 (+15分押)');
});
