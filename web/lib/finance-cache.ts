import type {
  RecurringBill,
  Summary,
  Transaction,
  TransactionList,
  TransactionQuery,
} from "./types";
import { karachiMonthKey } from "./date";
import type { LedgerTotalsDelta } from "./ledger";

export const FINANCE_CACHE_FRESH_MS = 30_000;
const VERSION = 1;
const MAX_TRANSACTION_QUERIES = 20;
const storageKey = (userID: string) => `rakam.finance.v${VERSION}.${userID}`;

export type CacheEntry<T> = { data: T; updatedAt: number };
export type FinanceCache = {
  version: 1;
  summary: CacheEntry<Summary> | null;
  transactions: Record<string, CacheEntry<TransactionList>>;
};

export function emptyFinanceCache(): FinanceCache {
  return { version: VERSION, summary: null, transactions: {} };
}

export function readFinanceCache(
  storage: Storage,
  userID: string,
): FinanceCache {
  const raw = storage.getItem(storageKey(userID));
  if (!raw) return emptyFinanceCache();
  try {
    const value: unknown = JSON.parse(raw);
    return isFinanceCache(value) ? value : emptyFinanceCache();
  } catch {
    return emptyFinanceCache();
  }
}

export function writeFinanceCache(
  storage: Storage,
  userID: string,
  cache: FinanceCache,
): void {
  storage.setItem(storageKey(userID), JSON.stringify(cache));
}

export function clearFinanceCache(storage: Storage, userID: string): void {
  storage.removeItem(storageKey(userID));
}

export function isFresh(entry: CacheEntry<unknown>, now = Date.now()): boolean {
  return now - entry.updatedAt < FINANCE_CACHE_FRESH_MS;
}

export function transactionQueryKey(query: TransactionQuery): string {
  const params = new URLSearchParams();
  if (query.kind) params.set("kind", query.kind);
  if (query.month) params.set("month", query.month);
  if (query.category_id) params.set("category_id", query.category_id);
  if (query.q) params.set("q", query.q);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  return params.toString();
}

export function withTransactionEntry(
  cache: FinanceCache,
  key: string,
  entry: CacheEntry<TransactionList>,
): FinanceCache {
  const transactions = { ...cache.transactions, [key]: entry };
  const keys = Object.keys(transactions);
  if (keys.length > MAX_TRANSACTION_QUERIES) {
    keys
      .sort((a, b) => transactions[a].updatedAt - transactions[b].updatedAt)
      .slice(0, keys.length - MAX_TRANSACTION_QUERIES)
      .forEach((oldest) => delete transactions[oldest]);
  }
  return { ...cache, transactions };
}

export function invalidateFinanceCache(cache: FinanceCache): FinanceCache {
  return {
    ...cache,
    summary: cache.summary ? { ...cache.summary, updatedAt: 0 } : null,
    transactions: Object.fromEntries(
      Object.entries(cache.transactions).map(([key, entry]) => [
        key,
        { ...entry, updatedAt: 0 },
      ]),
    ),
  };
}

/**
 * Apply a server-confirmed save to the cached dashboard before its background
 * refresh finishes. `previous === undefined` means create; `null` means an
 * edit whose old row was not cached, so totals must wait for the server.
 */
export function withSavedTransaction(
  cache: FinanceCache,
  saved: Transaction,
  previous: Transaction | null | undefined,
): FinanceCache {
  if (!cache.summary) return invalidateFinanceCache(cache);
  let summary = cache.summary.data;
  if (previous === undefined) {
    summary = applyToSummaryTotals(summary, saved, 1);
  } else if (previous !== null) {
    summary = applyToSummaryTotals(summary, previous, -1);
    summary = applyToSummaryTotals(summary, saved, 1);
  }
  summary = {
    ...summary,
    recent_transactions: upsertRecent(summary.recent_transactions, saved),
  };
  return invalidateFinanceCache({
    ...cache,
    summary: { data: summary, updatedAt: 0 },
  });
}

export function withoutDeletedTransaction(
  cache: FinanceCache,
  deleted: Transaction,
): FinanceCache {
  if (!cache.summary) return invalidateFinanceCache(cache);
  const summary = applyToSummaryTotals(cache.summary.data, deleted, -1);
  return invalidateFinanceCache({
    ...cache,
    summary: {
      data: {
        ...summary,
        recent_transactions: summary.recent_transactions.filter(
          (transaction) => transaction.id !== deleted.id,
        ),
      },
      updatedAt: 0,
    },
  });
}

/**
 * A debt entry changes the "money on the street" totals without changing an
 * expense or income total. Apply the server-confirmed delta immediately, then
 * reconcile Summary in the background.
 */
export function withLedgerTotalsDelta(
  cache: FinanceCache,
  totalsDelta: LedgerTotalsDelta,
): FinanceCache {
  if (
    !cache.summary ||
    (totalsDelta.owedToMePaisa === 0 && totalsDelta.iOwePaisa === 0)
  ) {
    return cache;
  }
  const summary = cache.summary.data;
  return invalidateFinanceCache({
    ...cache,
    summary: {
      data: {
        ...summary,
        owed_to_me_paisa:
          summary.owed_to_me_paisa + totalsDelta.owedToMePaisa,
        i_owe_paisa: summary.i_owe_paisa + totalsDelta.iOwePaisa,
        net_owed_paisa:
          summary.net_owed_paisa +
          totalsDelta.owedToMePaisa -
          totalsDelta.iOwePaisa,
      },
      updatedAt: 0,
    },
  });
}

function applyToSummaryTotals(
  summary: Summary,
  transaction: Transaction,
  direction: 1 | -1,
): Summary {
  if (karachiMonthKey(new Date(transaction.occurred_at)) !== summary.month) {
    return summary;
  }
  const amount = transaction.amount_paisa * direction;
  if (transaction.kind === "income") {
    return {
      ...summary,
      income_paisa: summary.income_paisa + amount,
      net_paisa: summary.net_paisa + amount,
    };
  }
  return {
    ...summary,
    expense_paisa: summary.expense_paisa + amount,
    net_paisa: summary.net_paisa - amount,
  };
}

function upsertRecent(
  recent: Transaction[],
  saved: Transaction,
): Transaction[] {
  return [...recent.filter((transaction) => transaction.id !== saved.id), saved]
    .sort((a, b) => {
      const timeOrder =
        new Date(b.occurred_at).getTime() - new Date(a.occurred_at).getTime();
      return timeOrder === 0 ? b.id.localeCompare(a.id) : timeOrder;
    })
    .slice(0, 5);
}

function isFinanceCache(value: unknown): value is FinanceCache {
  if (!value || typeof value !== "object") return false;
  const cache = value as Partial<FinanceCache>;
  if (cache.version !== VERSION) return false;
  if (cache.summary !== null && !isEntry(cache.summary, isSummary))
    return false;
  if (!cache.transactions || typeof cache.transactions !== "object")
    return false;
  return Object.values(cache.transactions).every((entry) =>
    isEntry(entry, isTransactionList),
  );
}

function isEntry<T>(
  value: unknown,
  isData: (data: unknown) => data is T,
): value is CacheEntry<T> {
  if (!value || typeof value !== "object") return false;
  const entry = value as Partial<CacheEntry<T>>;
  return (
    typeof entry.updatedAt === "number" &&
    Number.isFinite(entry.updatedAt) &&
    isData(entry.data)
  );
}

function isTransaction(value: unknown): value is Transaction {
  if (!value || typeof value !== "object") return false;
  const t = value as Partial<Transaction>;
  return (
    typeof t.id === "string" &&
    (t.kind === "expense" || t.kind === "income") &&
    isInteger(t.amount_paisa) &&
    typeof t.occurred_at === "string" &&
    (t.category_id === null || typeof t.category_id === "string") &&
    (t.description === null || typeof t.description === "string") &&
    (t.recurring_bill_id === null || typeof t.recurring_bill_id === "string") &&
    typeof t.created_at === "string" &&
    typeof t.updated_at === "string"
  );
}

function isTransactionList(value: unknown): value is TransactionList {
  if (!value || typeof value !== "object") return false;
  const list = value as Partial<TransactionList>;
  return (
    Array.isArray(list.transactions) &&
    list.transactions.every(isTransaction) &&
    isInteger(list.total) &&
    isInteger(list.expense_paisa) &&
    isInteger(list.limit) &&
    isInteger(list.offset)
  );
}

function isSummary(value: unknown): value is Summary {
  if (!value || typeof value !== "object") return false;
  const summary = value as Partial<Summary>;
  return (
    typeof summary.month === "string" &&
    isInteger(summary.income_paisa) &&
    isInteger(summary.expense_paisa) &&
    isInteger(summary.net_paisa) &&
    isInteger(summary.days_remaining) &&
    isInteger(summary.budget_limit_paisa) &&
    isInteger(summary.budget_spent_paisa) &&
    isInteger(summary.owed_to_me_paisa) &&
    isInteger(summary.i_owe_paisa) &&
    isInteger(summary.net_owed_paisa) &&
    Array.isArray(summary.upcoming_bills) &&
    summary.upcoming_bills.every(isUpcomingBill) &&
    Array.isArray(summary.recent_transactions) &&
    summary.recent_transactions.every(isTransaction)
  );
}

function isUpcomingBill(value: unknown): boolean {
  if (!value || typeof value !== "object") return false;
  const upcoming = value as { bill?: unknown; due_at?: unknown };
  return typeof upcoming.due_at === "string" && isRecurringBill(upcoming.bill);
}

function isRecurringBill(value: unknown): value is RecurringBill {
  if (!value || typeof value !== "object") return false;
  const bill = value as Partial<RecurringBill>;
  return (
    typeof bill.id === "string" &&
    typeof bill.name === "string" &&
    isInteger(bill.amount_paisa) &&
    (bill.category_id === null || typeof bill.category_id === "string") &&
    isInteger(bill.day_of_month) &&
    typeof bill.is_active === "boolean" &&
    (bill.last_generated_month === null ||
      typeof bill.last_generated_month === "string") &&
    typeof bill.created_at === "string" &&
    typeof bill.updated_at === "string"
  );
}

function isInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value);
}
