import test from 'node:test';
import assert from 'node:assert/strict';
import {
  calculateExpenseTotal,
  defaultSessionExpense,
  DRINK_FEE_OPTIONS,
  CHEKI_SIGNED_PRICE,
  CHEKI_REGULAR_PRICE,
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
