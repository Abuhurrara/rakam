/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const load = require('./load-ts.cjs');

const {
  balanceMeta,
  categoryKindForDirection,
  commonDirection,
  splitDebtEntries,
} = load('lib/ledger.ts');
const { createDebtEntry, settleDebtEntry } = load('lib/api.ts');

const theyOwe = {
  id: 'one', person_id: 'person', direction: 'they_owe', amount_paisa: 500000,
  description: 'Dinner', incurred_at: '2026-09-19T12:00:00+05:00', settled_at: null,
  created_at: '', updated_at: '',
};
const iOwe = { ...theyOwe, id: 'two', direction: 'i_owe', settled_at: '2026-09-20T12:00:00+05:00' };

for (const tc of [
  { balance: 1, label: 'Owes you', tone: 'owed' },
  { balance: -1, label: 'You owe', tone: 'owing' },
  { balance: 0, label: 'Settled up', tone: 'settled' },
]) test(`balance ${tc.balance} has the right label`, () => {
  assert.deepEqual(balanceMeta(tc.balance), { label: tc.label, tone: tc.tone });
});

test('entries stay partitioned into outstanding then settled history', () => {
  assert.deepEqual(splitDebtEntries([theyOwe, iOwe]), {
    unsettled: [theyOwe],
    settled: [iOwe],
  });
});

test('a settlement category uses the transaction kind for its own direction', () => {
  assert.equal(categoryKindForDirection('they_owe'), 'income');
  assert.equal(categoryKindForDirection('i_owe'), 'expense');
});

test('mixed settle-all batches never choose one shared category', () => {
  assert.equal(commonDirection([theyOwe]), 'they_owe');
  assert.equal(commonDirection([theyOwe, iOwe]), null);
});

test('Ledger preserves the raw amount string on create and encodes the person ID', async () => {
  const originalFetch = global.fetch;
  let received;
  global.fetch = async (path, init) => {
    received = { path, init };
    return new Response(JSON.stringify(theyOwe), { status: 201 });
  };
  try {
    await createDebtEntry('person/one', {
      direction: 'they_owe', amount: '1250.50', description: 'Lunch',
      incurred_at: '2026-09-19T12:00:00+05:00',
    });
    assert.equal(received.path, '/api/people/person%2Fone/entries');
    assert.deepEqual(JSON.parse(received.init.body), {
      direction: 'they_owe', amount: '1250.50', description: 'Lunch',
      incurred_at: '2026-09-19T12:00:00+05:00',
    });
  } finally {
    global.fetch = originalFetch;
  }
});

test('settling normally sends no category and records no transaction', async () => {
  const originalFetch = global.fetch;
  let received;
  global.fetch = async (path, init) => {
    received = { path, init };
    return new Response(JSON.stringify({ debt_entry: theyOwe, transaction: null }), { status: 200 });
  };
  try {
    await settleDebtEntry('entry one', { create_transaction: false });
    assert.equal(received.path, '/api/debt-entries/entry%20one/settle');
    assert.deepEqual(JSON.parse(received.init.body), { create_transaction: false });
  } finally {
    global.fetch = originalFetch;
  }
});
