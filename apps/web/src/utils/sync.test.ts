import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeSyncData } from './sync.ts';
import type { SessionExpense } from './expense.ts';

function makeExpense(ticketFee: number, updatedAt: string): SessionExpense {
  return {
    ticketFee,
    ticketLabel: '通常',
    drinkFee: 600,
    chekiSignedCount: 1,
    chekiRegularCount: 0,
    memo: '',
    updatedAt,
  };
}

test('mergeSyncData resolves conflicts using newer updatedAt and unions distinct sessions', () => {
  const localExpenses: Record<string, SessionExpense> = {
    '20261101-1': makeExpense(2500, '2026-11-23T10:00:00.000Z'),
    '20261103-1': makeExpense(3000, '2026-11-23T08:00:00.000Z'),
  };
  const cloudExpenses: Record<string, SessionExpense> = {
    '20261101-1': makeExpense(4000, '2026-11-23T09:00:00.000Z'), // older than local
    '20261103-1': makeExpense(4500, '2026-11-23T11:00:00.000Z'), // newer than local
    '20261107-1': makeExpense(2000, '2026-11-23T07:00:00.000Z'), // cloud-only
  };

  const result = mergeSyncData(
    localExpenses,
    cloudExpenses,
    ['20261101-1'],
    ['20261107-1'],
  );

  assert.equal(result.merged.expenses['20261101-1'].ticketFee, 2500);
  assert.equal(result.merged.expenses['20261103-1'].ticketFee, 4500);
  assert.equal(result.merged.expenses['20261107-1'].ticketFee, 2000);
  assert.deepEqual(result.merged.attended.sort(), ['20261101-1', '20261107-1']);
  assert.equal(result.localChanged, true);
  assert.equal(result.cloudNeedsPush, true);
});
