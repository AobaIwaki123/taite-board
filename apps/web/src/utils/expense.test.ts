import test from 'node:test';
import assert from 'node:assert/strict';
import {
  calculateExpenseTotal,
  defaultSessionExpense,
  parseTicketFee,
  isIncrementalFee,
  calculateMonthExpenseSummary,
  DRINK_FEE_OPTIONS,
  CHEKI_SIGNED_PRICE,
  CHEKI_REGULAR_PRICE,
  type SessionExpense,
} from './expense.ts';

test('constants match idol standards', () => {
  assert.equal(CHEKI_SIGNED_PRICE, 2000);
  assert.equal(CHEKI_REGULAR_PRICE, 1000);
  assert.deepEqual(DRINK_FEE_OPTIONS, [0, 500, 600, 700]);
});

test('calculateExpenseTotal computes sums correctly', () => {
  assert.equal(
    calculateExpenseTotal({
      ticketFee: 5000,
      drinkFee: 600,
      chekiSignedCount: 2, // 4000
      chekiRegularCount: 1, // 1000
    }),
    10600
  );

  assert.equal(
    calculateExpenseTotal({
      ticketFee: 0,
      drinkFee: 0,
      chekiSignedCount: 0,
      chekiRegularCount: 0,
    }),
    0
  );

  assert.equal(
    calculateExpenseTotal({
      ticketFee: 3000,
      drinkFee: 700,
      chekiSignedCount: 3, // 6000
      chekiRegularCount: 0,
    }),
    9700
  );
});

test('defaultSessionExpense provides safe initial structure', () => {
  const def = defaultSessionExpense();
  assert.equal(def.ticketFee, 0);
  assert.equal(def.drinkFee, 600);
  assert.equal(def.chekiSignedCount, 0);
  assert.equal(def.chekiRegularCount, 0);
  assert.equal(def.memo, '');
});

test('parseTicketFee correctly parses ticket strings with (+1D) and suffixes', () => {
  assert.equal(parseTicketFee('¥4,500'), 4500);
  assert.equal(parseTicketFee('¥2,500'), 2500);
  assert.equal(parseTicketFee('¥3,200 (+1D)'), 3200); // 11/1 Part 2 fix: must NOT be 32001
  assert.equal(parseTicketFee('¥3,700 (+1D)'), 3700); // 11/1 Part 2 fix: must NOT be 37001
  assert.equal(parseTicketFee('各+¥500 (+1D)'), 500);  // 11/1 Part 1 fix: must NOT be 5001
  assert.equal(parseTicketFee('¥3,000 (完売)'), 3000);
  assert.equal(parseTicketFee('¥0 (無料)'), 0);
  assert.equal(parseTicketFee('各+¥1,000 (+1D)'), 1000);
  assert.equal(parseTicketFee(3500), 3500);
  assert.equal(parseTicketFee(null), 0);
  assert.equal(parseTicketFee(undefined), 0);
});

test('isIncrementalFee detects diff-only fees', () => {
  assert.equal(isIncrementalFee('各+¥500 (+1D)'), true);
  assert.equal(isIncrementalFee('+¥500'), true);
  assert.equal(isIncrementalFee('各+¥1,000 (+1D)'), true);
  assert.equal(isIncrementalFee('¥3,200 (+1D)'), false);
  assert.equal(isIncrementalFee('前方エリア ¥4,500'), false);
});

test('calculateMonthExpenseSummary groups totals by month and week accurately', () => {
  const mockSessions = [
    { id: '20261101-1', date: '2026-11-01' },
    { id: '20261101-2', date: '2026-11-01' },
    { id: '20261103-1', date: '2026-11-03' },
    { id: '20261107-1', date: '2026-11-07' },
    { id: '20261112-1', date: '2026-11-12' },
  ];

  const mockExpenses: Record<string, SessionExpense> = {
    '20261101-1': {
      ticketFee: 2500,
      drinkFee: 600,
      chekiSignedCount: 1, // 2000
      chekiRegularCount: 0,
      memo: '1部',
    },
    '20261101-2': {
      ticketFee: 3200,
      drinkFee: 600,
      chekiSignedCount: 0,
      chekiRegularCount: 2, // 2000
      memo: '2部',
    },
    '20261103-1': {
      ticketFee: 5000,
      drinkFee: 600,
      chekiSignedCount: 2, // 4000
      chekiRegularCount: 1, // 1000
      memo: 'マリアージュ',
    },
  };

  const summary = calculateMonthExpenseSummary(
    2026,
    11,
    mockSessions,
    (id) => mockExpenses[id] || null
  );

  // Totals:
  // 11/01-1: 2500 + 600 + 2000 = 5100
  // 11/01-2: 3200 + 600 + 2000 = 5800 (11/01 day total = 10900)
  // 11/03-1: 5000 + 600 + 4000 + 1000 = 10600
  // Total = 10900 + 10600 = 21500
  assert.equal(summary.total, 21500);
  assert.equal(summary.ticketTotal, 2500 + 3200 + 5000); // 10700
  assert.equal(summary.drinkTotal, 600 + 600 + 600);    // 1800
  assert.equal(summary.chekiTotal, 9000);
  assert.equal(summary.chekiSignedCount, 3);
  assert.equal(summary.chekiRegularCount, 3);
  assert.equal(summary.recordedSessionCount, 3);

  // 2026-11-01 is a Sunday.
  // Week 0: 11/01 - 11/07. Contains 11/01 (10900) and 11/03 (10600) -> 21500
  assert.equal(summary.weeks[0].total, 21500);
  assert.equal(summary.weeks[0].startDay, 1);
  assert.equal(summary.weeks[0].endDay, 7);
});
