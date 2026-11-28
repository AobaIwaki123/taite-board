import test from 'node:test';
import assert from 'node:assert/strict';
import { formatTicketPricesInput, parseTicketPricesInput } from './ticket.ts';

test('formatTicketPricesInput formats structured ticket prices into an editable line', () => {
  assert.equal(
    formatTicketPricesInput([
      { tier: '前方', price: '¥4,000', amount: 4000 },
      { tier: '一般', price: '¥2,500 (+1D)', amount: 2500 },
      { tier: '女性', price: '無料', amount: 0 },
    ]),
    '前方 ¥4,000 / 一般 ¥2,500 (+1D) / 女性 無料',
  );
});

test('parseTicketPricesInput parses multi-tier slash-separated input back into structured prices', () => {
  assert.deepEqual(
    parseTicketPricesInput('前方 4,000円 / 一般 ¥2,500 (+1D) / 女性 無料 / 当日 +¥500'),
    [
      { tier: '前方', price: '¥4,000', amount: 4000 },
      { tier: '一般', price: '¥2,500 (+1D)', amount: 2500 },
      { tier: '女性', price: '無料', amount: 0 },
      { tier: '当日', price: '+¥500', amount: 500 },
    ],
  );
});

test('parseTicketPricesInput handles full-width input and single bare price strings', () => {
  assert.deepEqual(parseTicketPricesInput('前売　￥３２００'), [
    { tier: '前売', price: '¥3,200', amount: 3200 },
  ]);
  assert.deepEqual(parseTicketPricesInput('¥2,000'), [
    { tier: '通常', price: '¥2,000', amount: 2000 },
  ]);
});
