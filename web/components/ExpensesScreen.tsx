"use client";

import { useEffect, useMemo, useState } from "react";
import { ApiError, listTransactions } from "@/lib/api";
import { formatPaisa, sumPaisa } from "@/lib/money";
import {
  formatDayHeader,
  formatMonthLabel,
  formatTime,
  isFutureMonth,
  karachiDayKey,
  karachiMonthKey,
  shiftMonthKey,
} from "@/lib/date";
import { friendlyMessage } from "@/lib/useMutation";
import type { Transaction } from "@/lib/types";
import { useCategories } from "./CategoriesProvider";
import { useSaves, type PendingSave } from "./SavesProvider";
import { useAddSheet } from "./AddSheet";
import { Spinner } from "./Spinner";

const PAGE_SIZE = 50;

type Row =
  { kind: "saved"; t: Transaction } | { kind: "pending"; p: PendingSave };

export function ExpensesScreen() {
  const { byId, expense: expenseCategories } = useCategories();
  const { open } = useAddSheet();
  const { pending, updating, revision } = useSaves();

  const [month, setMonth] = useState(() => karachiMonthKey(new Date()));
  const [categoryId, setCategoryId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");

  const [saved, setSaved] = useState<Transaction[]>([]);
  const [apiTotal, setApiTotal] = useState(0);
  const [spent, setSpent] = useState<number | null>(null);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [reloadNonce, setReloadNonce] = useState(0);

  useEffect(() => {
    const id = setTimeout(() => {
      setDebouncedQuery(query.trim());
      setPage(0);
    }, 300);
    return () => clearTimeout(id);
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    setSpent(null);

    listTransactions(
      {
        month,
        kind: "expense",
        offset: page * PAGE_SIZE,
        category_id: categoryId ?? undefined,
        q: debouncedQuery || undefined,
        limit: PAGE_SIZE,
      },
      controller.signal,
    )
      .then((res) => {
        if (cancelled) return;
        setSaved(res.transactions);
        setApiTotal(res.total);
        setSpent(res.expense_paisa);
        if (page > 0 && res.transactions.length === 0) setPage(0);
      })
      .catch((err: unknown) => {
        // An abort is us changing filters, not a failure worth showing.
        if (cancelled) return;
        setError(
          err instanceof ApiError ? err : new ApiError("Failed to load"),
        );
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [month, categoryId, debouncedQuery, reloadNonce, revision, page]);

  // Every settled mutation reloads the active query instead of inserting a
  // row into a view whose category, search or month it may no longer match.
  const expenses = saved;

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") setReloadNonce((n) => n + 1);
    };
    window.addEventListener("online", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      window.removeEventListener("online", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, []);

  // Only show in-flight saves that belong in the view being looked at.
  const visiblePending = useMemo(
    () =>
      pending.filter((p) => {
        if (karachiMonthKey(new Date(p.input.occurred_at)) !== month) {
          return false;
        }
        if (categoryId && p.input.category_id !== categoryId) return false;
        if (
          debouncedQuery &&
          !(p.input.description ?? "")
            .toLowerCase()
            .includes(debouncedQuery.toLowerCase())
        ) {
          return false;
        }
        return true;
      }),
    [pending, month, categoryId, debouncedQuery],
  );

  const groups = useMemo(
    () => groupByDay(expenses, visiblePending),
    [expenses, visiblePending],
  );

  const filtered = categoryId !== null || debouncedQuery !== "";
  const truncated = apiTotal > PAGE_SIZE;

  return (
    <div className="px-4 pt-4">
      <MonthHeader
        month={month}
        onChange={(next) => {
          setMonth(next);
          setPage(0);
        }}
      />

      <div className="mt-3 rounded-2xl border border-line bg-paper-raised px-4 py-3.5">
        <p className="text-label uppercase tracking-widest text-ink-faint">
          {filtered ? "Spent, filtered" : "Spent this month"}
        </p>
        <p className="tabular mt-1 text-money-lg font-semibold text-ink">
          {spent === null ? "—" : formatPaisa(spent)}
        </p>
        {truncated ? (
          <p className="mt-1 text-xs text-ink-faint">
            Showing {page * PAGE_SIZE + 1}–
            {Math.min((page + 1) * PAGE_SIZE, apiTotal)} of {apiTotal}. Total
            includes all matching expenses.
          </p>
        ) : null}
      </div>

      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search notes"
        aria-label="Search notes"
        className="mt-3 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3.5 text-sm text-ink placeholder:text-ink-faint focus:border-primary focus:outline-none"
      />

      <div className="no-scrollbar -mx-4 mt-3 flex gap-1.5 overflow-x-auto px-4 pb-1">
        <FilterChip
          label="All"
          active={categoryId === null}
          onClick={() => {
            setCategoryId(null);
            setPage(0);
          }}
        />
        {expenseCategories.map((c) => (
          <FilterChip
            key={c.id}
            label={`${c.icon} ${c.name}`}
            active={categoryId === c.id}
            onClick={() => {
              setCategoryId(categoryId === c.id ? null : c.id);
              setPage(0);
            }}
          />
        ))}
      </div>

      <div className="mt-4">
        {loading ? (
          <div className="flex justify-center py-12 text-ink-faint">
            <Spinner size={24} />
          </div>
        ) : error ? (
          <ErrorState
            message={friendlyMessage(error)}
            onRetry={() => setReloadNonce((n) => n + 1)}
          />
        ) : groups.length === 0 ? (
          <EmptyState filtered={filtered} />
        ) : (
          groups.map((group) => (
            <section key={group.dayKey} className="mb-5">
              <header className="mb-1.5 flex items-baseline justify-between border-b border-line pb-1.5">
                <h2 className="text-sm font-medium text-ink-soft">
                  {formatDayHeader(group.dayKey)}
                </h2>
                <span className="tabular text-sm text-ink-faint">
                  {formatPaisa(group.subtotal)}
                  {truncated ? " shown" : ""}
                </span>
              </header>
              <ul>
                {group.rows.map((row) =>
                  row.kind === "saved" ? (
                    <TransactionRow
                      key={row.t.id}
                      transaction={row.t}
                      categoryName={
                        row.t.category_id
                          ? (byId.get(row.t.category_id)?.name ?? "Category")
                          : "Uncategorised"
                      }
                      icon={
                        row.t.category_id
                          ? (byId.get(row.t.category_id)?.icon ?? "•")
                          : "•"
                      }
                      saving={updating.has(row.t.id)}
                      onEdit={() => open({ type: "edit", transaction: row.t })}
                    />
                  ) : (
                    <PendingRow key={row.p.key} pending={row.p} byId={byId} />
                  ),
                )}
              </ul>
            </section>
          ))
        )}
      </div>
      {!loading && !error && truncated ? (
        <nav
          aria-label="Expense pages"
          className="flex items-center justify-between gap-3 py-4"
        >
          <button
            type="button"
            disabled={page === 0}
            onClick={() => setPage((p) => p - 1)}
            className="min-h-11 rounded-xl border border-line px-4 disabled:opacity-40"
          >
            Newer
          </button>
          <span className="text-sm">
            Page {page + 1} of {Math.ceil(apiTotal / PAGE_SIZE)}
          </span>
          <button
            type="button"
            disabled={(page + 1) * PAGE_SIZE >= apiTotal}
            onClick={() => setPage((p) => p + 1)}
            className="min-h-11 rounded-xl border border-line px-4 disabled:opacity-40"
          >
            Older
          </button>
        </nav>
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------- grouping */

type DayGroup = { dayKey: string; rows: Row[]; subtotal: number };

function groupByDay(
  transactions: readonly Transaction[],
  pending: readonly PendingSave[],
): DayGroup[] {
  const byDay = new Map<string, Row[]>();

  const push = (dayKey: string, row: Row) => {
    const list = byDay.get(dayKey);
    if (list) list.push(row);
    else byDay.set(dayKey, [row]);
  };

  // Pending first within a day, so a just-saved row is where the eye is.
  for (const p of pending)
    push(karachiDayKey(p.input.occurred_at), {
      kind: "pending",
      p,
    });
  for (const t of transactions)
    push(karachiDayKey(t.occurred_at), {
      kind: "saved",
      t,
    });

  return [...byDay.entries()]
    .sort((a, b) => (a[0] < b[0] ? 1 : -1))
    .map(([dayKey, rows]) => ({
      dayKey,
      rows,
      // Confirmed rows only — see the note on the month total.
      subtotal: sumPaisa(
        rows.flatMap((r) => (r.kind === "saved" ? [r.t.amount_paisa] : [])),
      ),
    }));
}

/* ----------------------------------------------------------------- rows */

function TransactionRow({
  transaction,
  categoryName,
  icon,
  saving,
  onEdit,
}: {
  transaction: Transaction;
  categoryName: string;
  icon: string;
  saving: boolean;
  onEdit: () => void;
}) {
  return (
    <li>
      <button
        type="button"
        onClick={onEdit}
        className={`flex min-h-14 w-full items-center gap-3 rounded-xl px-1 py-2 text-left transition-opacity active:bg-paper-sunken ${
          saving ? "opacity-45" : ""
        }`}
      >
        <span
          aria-hidden="true"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-paper-sunken text-base"
        >
          {icon}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-ink">
            {transaction.description || categoryName}
          </span>
          <span className="block truncate text-xs text-ink-faint">
            {transaction.description ? `${categoryName} · ` : ""}
            {formatTime(transaction.occurred_at)}
            {transaction.recurring_bill_id ? " · bill" : ""}
          </span>
        </span>
        <span className="tabular shrink-0 text-money font-medium text-brick">
          {formatPaisa(transaction.amount_paisa)}
        </span>
      </button>
    </li>
  );
}

/** An in-flight save: visible immediately, dimmed until the server agrees. */
function PendingRow({
  pending,
  byId,
}: {
  pending: PendingSave;
  byId: Map<string, { name: string; icon: string }>;
}) {
  const category = pending.input.category_id
    ? byId.get(pending.input.category_id)
    : undefined;

  return (
    <li>
      <div className="flex min-h-14 w-full items-center gap-3 px-1 py-2 opacity-45">
        <span
          aria-hidden="true"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-paper-sunken text-base"
        >
          {category?.icon ?? "•"}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-ink">
            {pending.input.description || category?.name || "Uncategorised"}
          </span>
          <span className="flex items-center gap-1.5 text-xs text-ink-faint">
            <Spinner size={11} />
            Saving
          </span>
        </span>
        {/* The raw string the user typed — still not parsed, even to show. */}
        <span className="tabular shrink-0 text-money font-medium text-ink-soft">
          Rs {pending.input.amount}
        </span>
      </div>
    </li>
  );
}

/* -------------------------------------------------------------- chrome */

function MonthHeader({
  month,
  onChange,
}: {
  month: string;
  onChange: (next: string) => void;
}) {
  const nextMonth = shiftMonthKey(month, 1);
  const canGoForward = !isFutureMonth(nextMonth);

  return (
    <div className="flex items-center justify-between">
      <button
        type="button"
        aria-label="Previous month"
        onClick={() => onChange(shiftMonthKey(month, -1))}
        className="flex h-11 w-11 items-center justify-center rounded-full text-ink-soft active:bg-paper-sunken"
      >
        <Chevron direction="left" />
      </button>
      <h1 className="text-base font-semibold text-ink">
        {formatMonthLabel(month)}
      </h1>
      <button
        type="button"
        aria-label="Next month"
        disabled={!canGoForward}
        onClick={() => onChange(nextMonth)}
        className="flex h-11 w-11 items-center justify-center rounded-full text-ink-soft active:bg-paper-sunken disabled:opacity-25"
      >
        <Chevron direction="right" />
      </button>
    </div>
  );
}

function Chevron({ direction }: { direction: "left" | "right" }) {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <path
        d={direction === "left" ? "m14.5 6-6 6 6 6" : "m9.5 6 6 6-6 6"}
        stroke="currentColor"
        strokeWidth="1.9"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function FilterChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`min-h-11 shrink-0 whitespace-nowrap rounded-full border px-3 text-sm ${
        active
          ? "border-primary bg-primary-tint font-medium text-ink"
          : "border-line bg-paper-raised text-ink-soft"
      }`}
    >
      {label}
    </button>
  );
}

function EmptyState({ filtered }: { filtered: boolean }) {
  return (
    <div className="rounded-2xl border border-dashed border-line-strong px-5 py-10 text-center">
      <p className="text-sm text-ink-soft">
        {filtered ? "Nothing matches that." : "No expenses this month yet."}
      </p>
      {!filtered ? (
        <p className="mt-1 text-xs text-ink-faint">
          Tap + to add your first one.
        </p>
      ) : null}
    </div>
  );
}

function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div
      role="alert"
      className="rounded-2xl border border-brick/30 bg-brick-tint px-5 py-8 text-center"
    >
      <p className="text-sm font-medium text-brick">Couldn&apos;t load these</p>
      <p className="mt-1 text-xs text-ink-soft">{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-4 min-h-11 rounded-full border border-line-strong px-5 text-sm font-medium text-ink"
      >
        Try again
      </button>
    </div>
  );
}
