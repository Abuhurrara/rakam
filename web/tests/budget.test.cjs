/* eslint-disable @typescript-eslint/no-require-imports -- Existing Node test runner. */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const load = require('./load-ts.cjs');
const { listBudgets, upsertBudget, deleteBudget, ApiError } = load('lib/api.ts');
const { shiftMonthKey, isFutureMonth } = load('lib/date.ts');
const { budgetState } = load('lib/home.ts');
const {
  emptyFinanceCache, isFresh, invalidateFinanceCache, readFinanceCache,
  withBudgetMonth, withBudgetChange, withSavedTransaction, withoutDeletedTransaction, writeFinanceCache,
} = load('lib/finance-cache.ts');

const category = {
  id: 'food', name: 'Food', kind: 'expense', icon: '🍔', color: '#8B4513',
  sort_order: 0, is_archived: false, created_at: '', updated_at: '',
};
const budget = {
  id: 'budget-1', category_id: 'food', month: '2026-09', limit_paisa: 100000,
  created_at: '', updated_at: '',
};
const rows = [{ category, budget: null, spent_paisa: 25000 }];
const summary = {
  month: '2026-09', income_paisa: 0, expense_paisa: 40000, net_paisa: -40000,
  days_remaining: 7, budget_limit_paisa: 0, budget_spent_paisa: 0,
  owed_to_me_paisa: 0, i_owe_paisa: 0, net_owed_paisa: 0,
  upcoming_bills: [], recent_transactions: [],
};
function storage() {
  const map = new Map();
  return {
    getItem: (key) => map.get(key) ?? null,
    setItem: (key, value) => map.set(key, value),
    removeItem: (key) => map.delete(key),
  };
}

test('month controls stop at the current Karachi month', () => {
  const now = new Date('2026-09-23T20:30:00Z'); // September 24 in Karachi
  assert.equal(shiftMonthKey('2026-01', -1), '2025-12');
  assert.equal(shiftMonthKey('2026-08', 1), '2026-09');
  assert.equal(isFutureMonth('2026-09', now), false);
  assert.equal(isFutureMonth('2026-10', now), true);
});

test('warning colours use exact 80% and above 100% boundaries', () => {
  for (const [spent, tone] of [[7999, 'primary'], [8000, 'gold'], [10000, 'gold'], [10001, 'brick']]) {
    assert.equal(budgetState(spent, 10000).tone, tone);
  }
});

test('budget snapshots are account scoped, month keyed, and keep older finance snapshots', () => {
  const disk = storage();
  const cache = withBudgetMonth(emptyFinanceCache(), '2026-09', { data: rows, updatedAt: 10000 });
  writeFinanceCache(disk, 'alice', cache);
  assert.deepEqual(readFinanceCache(disk, 'alice').budgets['2026-09'].data, rows);
  assert.equal(readFinanceCache(disk, 'bob').budgets['2026-09'], undefined);
  assert.equal(isFresh(cache.budgets['2026-09'], 39999), true);
  assert.equal(invalidateFinanceCache(cache).budgets['2026-09'].updatedAt, 0);

  const oldDisk = storage();
  oldDisk.setItem('rakam.finance.v1.alice', JSON.stringify({ version: 1, summary: null, transactions: {} }));
  assert.deepEqual(readFinanceCache(oldDisk, 'alice').budgets, {});
});

test('confirmed save, edit, and remove update Budget and Home without another GET', () => {
  let cache = withBudgetMonth({ ...emptyFinanceCache(), summary: { data: summary, updatedAt: 10000 } },
    '2026-09', { data: rows, updatedAt: 10000 });
  cache = withBudgetChange(cache, '2026-09', 'food', budget);
  assert.equal(cache.budgets['2026-09'].data[0].budget.id, 'budget-1');
  assert.equal(cache.summary.data.budget_limit_paisa, 100000);
  assert.equal(cache.summary.data.budget_spent_paisa, 25000);
  assert.equal(cache.summary.updatedAt, 0);

  cache = withBudgetChange(cache, '2026-09', 'food', { ...budget, limit_paisa: 150000 });
  assert.equal(cache.summary.data.budget_limit_paisa, 150000);
  assert.equal(cache.summary.data.budget_spent_paisa, 25000);
  cache = withBudgetChange(cache, '2026-09', 'food', null);
  assert.equal(cache.summary.data.budget_limit_paisa, 0);
  assert.equal(cache.summary.data.budget_spent_paisa, 0);
  assert.equal(cache.budgets['2026-09'].data[0].budget, null);
  assert.equal(cache.budgets['2026-09'].updatedAt, 0);
});

test('confirmed expense changes update budgeted spending but not unbudgeted spending', () => {
  const budgetedRows = [{ ...rows[0], budget }];
  const expense = {
    id: 'food-tx', kind: 'expense', amount_paisa: 500,
    category_id: 'food', description: null,
    occurred_at: '2026-09-23T12:00:00+05:00', recurring_bill_id: null,
    created_at: '', updated_at: '',
  };
  let cache = withBudgetMonth({ ...emptyFinanceCache(), summary: {
    data: { ...summary, budget_limit_paisa: 100000, budget_spent_paisa: 25000 }, updatedAt: 10000,
  } }, '2026-09', { data: budgetedRows, updatedAt: 10000 });
  cache = withSavedTransaction(cache, expense, undefined);
  assert.equal(cache.summary.data.budget_spent_paisa, 25500);
  const edited = { ...expense, amount_paisa: 700 };
  cache = withSavedTransaction(cache, edited, expense);
  assert.equal(cache.summary.data.budget_spent_paisa, 25700);
  cache = withoutDeletedTransaction(cache, edited);
  assert.equal(cache.summary.data.budget_spent_paisa, 25000);
  cache = withSavedTransaction(cache, { ...expense, id: 'travel-tx', category_id: 'travel' }, undefined);
  assert.equal(cache.summary.data.budget_spent_paisa, 25000);
});

test('an expense save invalidates cached spending for every month', () => {
  let cache = withBudgetMonth(emptyFinanceCache(), '2026-09', { data: rows, updatedAt: 10000 });
  cache = withBudgetMonth(cache, '2026-08', { data: rows, updatedAt: 10000 });
  const expense = {
    id: 'tx', kind: 'expense', amount_paisa: 500, category_id: 'food',
    description: null, occurred_at: '2026-09-23T12:00:00+05:00',
    recurring_bill_id: null, created_at: '', updated_at: '',
  };
  const next = withSavedTransaction(cache, expense, undefined);
  assert.equal(next.budgets['2026-09'].updatedAt, 0);
  assert.equal(next.budgets['2026-08'].updatedAt, 0);
});

test('Budget API sends the selected month and raw amount; errors keep the caller in control', async () => {
  const originalFetch = global.fetch;
  const requests = [];
  global.fetch = async (path, init) => {
    requests.push({ path, init });
    if (init.method === 'PUT' && requests.length > 2) {
      return new Response(JSON.stringify({ error: 'invalid budget' }), { status: 400 });
    }
    if (init.method === 'DELETE') return new Response(null, { status: 204 });
    return new Response(JSON.stringify(init.method === 'PUT' ? budget : rows), { status: 200 });
  };
  try {
    await listBudgets('2026-08');
    await upsertBudget({ category_id: 'food', month: '2026-08', limit: '1250.50' });
    await deleteBudget('id/with space');
    assert.equal(requests[0].path, '/api/budgets?month=2026-08');
    assert.equal(requests[1].init.method, 'PUT');
    assert.deepEqual(JSON.parse(requests[1].init.body), {
      category_id: 'food', month: '2026-08', limit: '1250.50',
    });
    assert.equal(requests[2].path, '/api/budgets/id%2Fwith%20space');
    await assert.rejects(upsertBudget({ category_id: 'food', month: '2026-08', limit: '0' }),
      (err) => err instanceof ApiError && err.status === 400);
  } finally {
    global.fetch = originalFetch;
  }
});
