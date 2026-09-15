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
  withTransactionEntry,
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
