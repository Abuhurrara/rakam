/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const load = require('./load-ts.cjs');

const {
  balanceMeta,
  categoryKindForDirection,
  commonDirection,
  debtEntryBalancePaisa,
  debtEntryTotals,
  replaceDebtEntries,
  splitDebtEntries,
} = load('lib/ledger.ts');
const { createDebtEntry, settleDebtEntry } = load('lib/api.ts');
const {
  emptyLedgerCache,
  isLedgerFresh,
  readLedgerCache,
  withEntryList,
  writeLedgerCache,
} = load('lib/ledger-cache.ts');

const theyOwe = {
  id: 'one', person_id: 'person', direction: 'they_owe', amount_paisa: 500000,
  description: 'Dinner', incurred_at: '2026-09-19T12:00:00+05:00', settled_at: null,
  created_at: '', updated_at: '',
};
const iOwe = { ...theyOwe, id: 'two', direction: 'i_owe', settled_at: '2026-09-20T12:00:00+05:00' };

function storage() {
  const map = new Map();
  return {
    getItem: (key) => map.get(key) ?? null,
    setItem: (key, value) => map.set(key, value),
    removeItem: (key) => map.delete(key),
  };
}

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

test('server-confirmed settlements update a cached balance without waiting for a refetch', () => {
  assert.equal(debtEntryBalancePaisa(theyOwe), 500000);
  assert.equal(debtEntryBalancePaisa(iOwe), 0);
  const settled = { ...theyOwe, settled_at: '2026-09-20T12:00:00+05:00' };
  const result = replaceDebtEntries([theyOwe, iOwe], [settled]);
  assert.deepEqual(result.entries, [settled, iOwe]);
  assert.equal(result.balanceDelta, -500000);
  assert.deepEqual(debtEntryTotals(theyOwe), {
    owedToMePaisa: 500000,
    iOwePaisa: 0,
  });
  assert.deepEqual(result.totalsDelta, {
    owedToMePaisa: -500000,
    iOwePaisa: 0,
  });
});

test('Ledger cache is account-scoped and a fresh visit skips a repeat loader', () => {
  const disk = storage();
  const people = [{
    id: 'person', name: 'Usman', phone: null, notes: null, balance_paisa: 500000,
    created_at: '2026-09-19T12:00:00Z', updated_at: '2026-09-19T12:00:00Z',
  }];
  const entries = withEntryList(
    { ...emptyLedgerCache(), people: { data: people, updatedAt: 10_000 } },
    'person',
    { data: [theyOwe], updatedAt: 10_000 },
  );
  writeLedgerCache(disk, 'alice', entries);
  const cached = readLedgerCache(disk, 'alice');
  assert.deepEqual(cached, entries);
  assert.deepEqual(readLedgerCache(disk, 'bob'), emptyLedgerCache());
  assert.equal(isLedgerFresh(cached.people, 39_999), true);
  assert.equal(isLedgerFresh(cached.entries.person, 40_000), false);
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
