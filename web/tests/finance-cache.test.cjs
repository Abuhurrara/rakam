/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require("node:test");
const assert = require("node:assert/strict");
const load = require("./load-ts.cjs");
const {
  clearFinanceCache,
  emptyFinanceCache,
  invalidateFinanceCache,
  isFresh,
  readFinanceCache,
  transactionQueryKey,
  withLedgerTotalsDelta,
  withSavedTransaction,
  withTransactionEntry,
  withoutDeletedTransaction,
  writeFinanceCache,
} = load("lib/finance-cache.ts");

function storage() {
  const map = new Map();
  return {
    get length() {
      return map.size;
    },
    key: (i) => [...map.keys()][i] ?? null,
    getItem: (key) => map.get(key) ?? null,
    setItem: (key, value) => map.set(key, value),
    removeItem: (key) => map.delete(key),
  };
}

const transactionList = {
  transactions: [
    {
      id: "tx-1",
      kind: "expense",
      amount_paisa: 12500,
      category_id: null,
      description: "Lunch",
      occurred_at: "2026-09-15T12:00:00+05:00",
      recurring_bill_id: null,
      created_at: "2026-09-15T12:00:00Z",
      updated_at: "2026-09-15T12:00:00Z",
    },
  ],
  total: 1,
  expense_paisa: 12500,
  limit: 50,
  offset: 0,
};

const summary = {
  month: "2026-09",
  income_paisa: 100000,
  expense_paisa: 25000,
  net_paisa: 75000,
  days_remaining: 15,
  budget_limit_paisa: 200000,
  budget_spent_paisa: 20000,
  owed_to_me_paisa: 0,
  i_owe_paisa: 0,
  net_owed_paisa: 0,
  upcoming_bills: [],
  recent_transactions: transactionList.transactions,
};

test("finance cache is versioned and isolated by account", () => {
  const disk = storage();
  const key = transactionQueryKey({
    kind: "expense",
    month: "2026-09",
    limit: 50,
    offset: 0,
  });
  const alice = withTransactionEntry(emptyFinanceCache(), key, {
    data: transactionList,
    updatedAt: 1000,
  });
  writeFinanceCache(disk, "alice", alice);
  assert.deepEqual(readFinanceCache(disk, "alice"), alice);
  assert.deepEqual(readFinanceCache(disk, "bob"), emptyFinanceCache());
  clearFinanceCache(disk, "alice");
  assert.deepEqual(readFinanceCache(disk, "alice"), emptyFinanceCache());
});

test("invalid cached financial data is ignored instead of reaching the UI", () => {
  const disk = storage();
  disk.setItem("rakam.finance.v1.alice", "{broken");
  assert.deepEqual(readFinanceCache(disk, "alice"), emptyFinanceCache());
  disk.setItem(
    "rakam.finance.v1.alice",
    JSON.stringify({
      version: 1,
      summary: null,
      transactions: { bad: { data: {}, updatedAt: 1 } },
    }),
  );
  assert.deepEqual(readFinanceCache(disk, "alice"), emptyFinanceCache());
});

test("fresh entries skip repeat requests and invalidation makes every view stale", () => {
  const key = transactionQueryKey({
    month: "2026-09",
    kind: "expense",
    offset: 0,
    limit: 50,
  });
  const cache = withTransactionEntry(emptyFinanceCache(), key, {
    data: transactionList,
    updatedAt: 10_000,
  });
  assert.equal(isFresh(cache.transactions[key], 39_999), true);
  assert.equal(isFresh(cache.transactions[key], 40_000), false);
  const invalidated = invalidateFinanceCache(cache);
  assert.equal(invalidated.transactions[key].updatedAt, 0);
  assert.equal(isFresh(invalidated.transactions[key], 40_000), false);
});

test("transaction query keys are stable and distinguish filters", () => {
  const base = transactionQueryKey({
    kind: "expense",
    month: "2026-09",
    limit: 50,
    offset: 0,
  });
  assert.equal(
    base,
    transactionQueryKey({
      kind: "expense",
      month: "2026-09",
      limit: 50,
      offset: 0,
    }),
  );
  assert.notEqual(
    base,
    transactionQueryKey({
      kind: "expense",
      month: "2026-09",
      category_id: "food",
      limit: 50,
      offset: 0,
    }),
  );
});

test("persistent query history is bounded to the twenty newest views", () => {
  let cache = emptyFinanceCache();
  for (let index = 0; index < 25; index++) {
    cache = withTransactionEntry(cache, `page=${index}`, {
      data: { ...transactionList, offset: index * 50 },
      updatedAt: index,
    });
  }
  assert.equal(Object.keys(cache.transactions).length, 20);
  assert.equal(cache.transactions["page=0"], undefined);
  assert.ok(cache.transactions["page=24"]);
});

test("server-confirmed create updates the cached Home total immediately", () => {
  const cache = {
    ...emptyFinanceCache(),
    summary: { data: summary, updatedAt: 10_000 },
  };
  const saved = {
    ...transactionList.transactions[0],
    id: "tx-2",
    amount_paisa: 5000,
    occurred_at: "2026-09-15T13:00:00+05:00",
  };
  const next = withSavedTransaction(cache, saved, undefined);
  assert.equal(next.summary.data.expense_paisa, 30000);
  assert.equal(next.summary.data.net_paisa, 70000);
  assert.equal(next.summary.data.recent_transactions[0].id, "tx-2");
  assert.equal(next.summary.updatedAt, 0);
});

test("edit and delete apply exact deltas before background reconciliation", () => {
  const cache = {
    ...emptyFinanceCache(),
    summary: { data: summary, updatedAt: 10_000 },
  };
  const previous = transactionList.transactions[0];
  const edited = { ...previous, amount_paisa: 15000 };
  const afterEdit = withSavedTransaction(cache, edited, previous);
  assert.equal(afterEdit.summary.data.expense_paisa, 27500);
  assert.equal(afterEdit.summary.data.net_paisa, 72500);

  const afterDelete = withoutDeletedTransaction(afterEdit, edited);
  assert.equal(afterDelete.summary.data.expense_paisa, 12500);
  assert.equal(afterDelete.summary.data.net_paisa, 87500);
  assert.equal(afterDelete.summary.data.recent_transactions.length, 0);
});

test("Ledger changes update the exact money-on-the-street total before revalidation", () => {
  const cache = {
    ...emptyFinanceCache(),
    summary: { data: summary, updatedAt: 10_000 },
  };
  const afterLend = withLedgerTotalsDelta(cache, {
    owedToMePaisa: 5000,
    iOwePaisa: 0,
  });
  assert.equal(afterLend.summary.data.owed_to_me_paisa, 5000);
  assert.equal(afterLend.summary.data.i_owe_paisa, 0);
  assert.equal(afterLend.summary.data.net_owed_paisa, 5000);
  assert.equal(afterLend.summary.updatedAt, 0);

  const afterBorrow = withLedgerTotalsDelta(afterLend, {
    owedToMePaisa: 0,
    iOwePaisa: 2000,
  });
  assert.equal(afterBorrow.summary.data.owed_to_me_paisa, 5000);
  assert.equal(afterBorrow.summary.data.i_owe_paisa, 2000);
  assert.equal(afterBorrow.summary.data.net_owed_paisa, 3000);

  const afterSettlement = withLedgerTotalsDelta(afterBorrow, {
    owedToMePaisa: -5000,
    iOwePaisa: 0,
  });
  assert.equal(afterSettlement.summary.data.owed_to_me_paisa, 0);
  assert.equal(afterSettlement.summary.data.i_owe_paisa, 2000);
  assert.equal(afterSettlement.summary.data.net_owed_paisa, -2000);
});
