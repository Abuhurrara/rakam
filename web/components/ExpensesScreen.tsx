"use client";

import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ApiError, listTransactions } from "@/lib/api";
import { formatPaisa, sumPaisa } from "@/lib/money";
import {
  formatDayHeader,
  formatMonthLabel,
  formatTime,
  isFutureMonth,
  karachiDayKey,
  karachiDateInputValue,
  karachiLast14DaysRange,
  karachiMonthKey,
  karachiWeekRange,
  shiftMonthKey,
} from "@/lib/date";
import { friendlyMessage } from "@/lib/useMutation";
import { isFresh, transactionQueryKey } from "@/lib/finance-cache";
import type {
  Category,
  Transaction,
  TransactionList,
  TransactionQuery,
} from "@/lib/types";
import { useCategories } from "./CategoriesProvider";
import { useSaves, type PendingSave } from "./SavesProvider";
import { useAddSheet } from "./AddSheet";
import { Spinner } from "./Spinner";
import { useFinanceData } from "./FinanceDataProvider";

const PAGE_SIZE = 50;
type Period = "month" | "week" | "14days" | "custom";

type Row =
  { kind: "saved"; t: Transaction } | { kind: "pending"; p: PendingSave };

export function ExpensesScreen() {
  const { byId, expense: expenseCategories } = useCategories();
  const { open } = useAddSheet();
  const { pending, updating, revision } = useSaves();
  const financeData = useFinanceData();

  const [month, setMonth] = useState(() => karachiMonthKey(new Date()));
  const [period, setPeriod] = useState<Period>("month");
  const [customFrom, setCustomFrom] = useState(
    () => `${karachiMonthKey(new Date())}-01`,
  );
  const [customTo, setCustomTo] = useState(() => karachiDateInputValue(new Date()));
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [categoryId, setCategoryId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");

  const [page, setPage] = useState(0);
  const queryParams = useMemo<TransactionQuery>(() => {
    const today = new Date();
    const range =
      period === "week"
        ? karachiWeekRange(today)
        : period === "14days"
          ? karachiLast14DaysRange(today)
          : null;
    return {
      ...(period === "month"
        ? { month }
        : period === "custom"
          ? { from: customFrom, to: customTo }
          : (range ?? {})),
      kind: "expense",
      offset: page * PAGE_SIZE,
      category_id: categoryId ?? undefined,
      q: debouncedQuery || undefined,
      limit: PAGE_SIZE,
    };
  },
    [month, period, customFrom, customTo, page, categoryId, debouncedQuery],
  );
  const cacheKey = transactionQueryKey(queryParams);
  const initialCached = financeData.readTransactions(cacheKey);
  const [result, setResult] = useState<{
    key: string;
    data: TransactionList;
  } | null>(() =>
    initialCached ? { key: cacheKey, data: initialCached.data } : null,
  );
  const [loading, setLoading] = useState(initialCached === null);
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
    const cached = financeData.readTransactions(cacheKey);
    if (cached) setResult({ key: cacheKey, data: cached.data });
    if (reloadNonce === 0 && cached && isFresh(cached)) {
      setLoading(false);
      setError(null);
      return () => controller.abort();
    }
    setLoading(true);
    setError(null);

    listTransactions(queryParams, controller.signal)
      .then((res) => {
        if (cancelled) return;
        financeData.writeTransactions(cacheKey, res);
        setResult({ key: cacheKey, data: res });
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
  }, [cacheKey, queryParams, reloadNonce, revision, page, financeData]);

  const cachedForView = financeData.readTransactions(cacheKey);
  const data =
    result?.key === cacheKey ? result.data : (cachedForView?.data ?? null);
  const expenses = useMemo(() => data?.transactions ?? [], [data]);
  const apiTotal = data?.total ?? 0;
  const spent = data?.expense_paisa ?? null;

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
        const day = karachiDayKey(p.input.occurred_at);
        const range =
          period === "week"
            ? karachiWeekRange(new Date())
            : period === "14days"
              ? karachiLast14DaysRange(new Date())
              : null;
        if (
          period === "month" &&
          karachiMonthKey(new Date(p.input.occurred_at)) !== month
        ) {
          return false;
        }
        if (period === "custom" && (day < customFrom || day > customTo)) {
          return false;
        }
        if (range && (day < range.from || day > range.to)) {
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
    [pending, month, period, customFrom, customTo, categoryId, debouncedQuery],
  );

  const groups = useMemo(
    () => groupByDay(expenses, visiblePending),
    [expenses, visiblePending],
  );

  const filtered =
    period !== "month" || categoryId !== null || debouncedQuery !== "";
  const truncated = apiTotal > PAGE_SIZE;
  const activeFilterCount = Number(period !== "month") + Number(categoryId !== null);
  const spentLabel =
    period === "week"
      ? "Spent this week"
      : period === "14days"
        ? "Spent in last 14 days"
        : period === "custom"
          ? `Spent · ${customFrom} to ${customTo}`
          : `Spent in ${formatMonthLabel(month)}`;

  return (
    <div className="px-4 pt-4">
      {period === "month" ? (
        <MonthHeader
          month={month}
          onChange={(next) => {
            setMonth(next);
            setPage(0);
          }}
        />
      ) : (
        <div className="flex min-h-11 items-center justify-center text-base font-semibold text-ink">
          {period === "week"
            ? "This week"
            : period === "14days"
              ? "Last 14 days"
              : "Custom dates"}
        </div>
      )}

      <div className="mt-3 rounded-2xl border border-line bg-paper-raised px-4 py-3.5">
        <p className="text-label uppercase tracking-widest text-ink-faint">
          {spentLabel}
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

      <div className="mt-3 flex items-center gap-2">
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search notes"
          aria-label="Search notes"
          className="min-h-11 min-w-0 flex-1 rounded-xl border border-line bg-paper-raised px-3.5 text-sm text-ink placeholder:text-ink-faint focus:border-primary focus:outline-none"
        />
        <button
          type="button"
          onClick={() => setFiltersOpen((open) => !open)}
          aria-expanded={filtersOpen}
          aria-controls="expense-filter-options"
          aria-label={`${filtersOpen ? "Hide" : "Show"} filters${activeFilterCount ? `, ${activeFilterCount} active` : ""}`}
          className={`flex min-h-11 shrink-0 items-center gap-2 rounded-xl border px-3 text-sm font-medium ${activeFilterCount ? "border-primary/40 bg-primary/10 text-primary" : "border-line bg-paper-raised text-ink-soft"}`}
        >
          <FilterIcon />
          Filters
          {activeFilterCount ? (
            <span className="tabular text-xs">{activeFilterCount}</span>
          ) : null}
        </button>
      </div>

      {activeFilterCount ? (
        <div className="no-scrollbar mt-2 flex gap-1.5 overflow-x-auto pb-0.5">
          {period !== "month" ? (
            <AppliedFilter
              label={
                period === "week"
                  ? "This week"
                  : period === "14days"
                    ? "Last 14 days"
                    : "Custom dates"
              }
              onRemove={() => {
                setPeriod("month");
                setPage(0);
              }}
            />
          ) : null}
          {categoryId ? (
            <AppliedFilter
              label={(() => {
                const category = expenseCategories.find(
                  (item) => item.id === categoryId,
                );
                return category ? `${category.icon} ${category.name}` : "Category";
              })()}
              onRemove={() => {
                setCategoryId(null);
                setPage(0);
              }}
            />
          ) : null}
        </div>
      ) : null}

      <ExpenseFilterSheet
        open={filtersOpen}
        onRequestClose={() => setFiltersOpen(false)}
        period={period}
        month={month}
        customFrom={customFrom}
        customTo={customTo}
        categories={expenseCategories}
        categoryId={categoryId}
        onPeriodChange={(next) => {
          setPeriod(next);
          setPage(0);
        }}
        onCustomFromChange={(next) => {
          setCustomFrom(next);
          setPage(0);
        }}
        onCustomToChange={(next) => {
          setCustomTo(next);
          setPage(0);
        }}
        onCategoryChange={(next) => {
          setCategoryId(next);
          setPage(0);
        }}
        onClear={() => {
          setPeriod("month");
          setCategoryId(null);
          setPage(0);
        }}
      />

      <div className="mt-4" aria-busy={loading}>
        {data && error ? (
          <div
            role="alert"
            className="mb-4 flex items-center justify-between gap-3 rounded-xl border border-brick/30 bg-brick-tint px-4 py-3"
          >
            <p className="text-sm text-brick">
              {friendlyMessage(error)} Saved data is still shown.
            </p>
            <button
              type="button"
              onClick={() => setReloadNonce((n) => n + 1)}
              className="min-h-11 shrink-0 px-2 text-sm font-semibold text-ink"
            >
              Retry
            </button>
          </div>
        ) : null}
        {!data && loading ? (
          <ExpenseListSkeleton />
        ) : !data && error ? (
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
      {data && truncated ? (
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

/** Mirrors the real day group so loading does not collapse the list area. */
function ExpenseListSkeleton() {
  return (
    <div aria-label="Loading expenses">
      <span className="sr-only" role="status">
        Loading expenses
      </span>
      <div aria-hidden="true" className="animate-pulse">
        <div className="mb-1.5 flex items-center justify-between border-b border-line pb-2">
          <div className="h-3.5 w-24 rounded bg-paper-sunken" />
          <div className="h-3.5 w-20 rounded bg-paper-sunken" />
        </div>
        <ul>
          {Array.from({ length: 5 }, (_, index) => (
            <li
              key={index}
              className="flex min-h-14 items-center gap-3 px-1 py-2"
            >
              <span className="h-9 w-9 shrink-0 rounded-full bg-paper-sunken" />
              <span className="min-w-0 flex-1 space-y-1.5">
                <span
                  className={`block h-3.5 rounded bg-paper-sunken ${
                    index % 2 === 0 ? "w-28" : "w-36"
                  }`}
                />
                <span className="block h-2.5 w-20 rounded bg-paper-sunken" />
              </span>
              <span className="h-4 w-20 shrink-0 rounded bg-paper-sunken" />
            </li>
          ))}
        </ul>
      </div>
    </div>
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

function AppliedFilter({
  label,
  onRemove,
}: {
  label: string;
  onRemove: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onRemove}
      aria-label={`Remove ${label} filter`}
      className="min-h-8 shrink-0 rounded-full border border-primary/30 bg-primary/10 px-3 text-xs font-medium text-primary"
    >
      {label} <span aria-hidden="true">×</span>
    </button>
  );
}

function CategoryOption({
  label,
  icon,
  selected,
  animationIndex,
  onClick,
}: {
  label: string;
  icon: string;
  selected: boolean;
  animationIndex: number;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      style={{ animationDelay: `${Math.min(animationIndex, 8) * 18}ms` }}
      className={`expense-category-option flex min-h-11 w-full items-center gap-3 rounded-xl px-3 text-left text-sm transition-[background-color,transform] duration-150 active:scale-[0.99] ${
        selected
          ? "bg-primary-tint font-medium text-ink"
          : "text-ink-soft hover:bg-paper-sunken"
      }`}
    >
      <span aria-hidden="true" className="w-5 text-center text-base">
        {icon}
      </span>
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {selected ? <CheckIcon /> : null}
    </button>
  );
}

function CategoryChevron({ expanded }: { expanded: boolean }) {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className={`transition-transform duration-150 ${expanded ? "rotate-180" : ""}`}
    >
      <path
        d="m7 10 5 5 5-5"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="m5 12 4 4L19 6"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ExpenseFilterSheet({
  open,
  onRequestClose,
  period,
  month,
  customFrom,
  customTo,
  categories,
  categoryId,
  onPeriodChange,
  onCustomFromChange,
  onCustomToChange,
  onCategoryChange,
  onClear,
}: {
  open: boolean;
  onRequestClose: () => void;
  period: Period;
  month: string;
  customFrom: string;
  customTo: string;
  categories: Category[];
  categoryId: string | null;
  onPeriodChange: (period: Period) => void;
  onCustomFromChange: (date: string) => void;
  onCustomToChange: (date: string) => void;
  onCategoryChange: (id: string | null) => void;
  onClear: () => void;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const categoryButtonRef = useRef<HTMLButtonElement>(null);
  const expectedCloseEventRef = useRef(false);
  const titleId = useId();
  const [categoryMenuOpen, setCategoryMenuOpen] = useState(false);
  const [closing, setClosing] = useState(false);

  function closeSheet() {
    if (closing) return;
    setCategoryMenuOpen(false);
    setClosing(true);
  }

  useLayoutEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) {
      expectedCloseEventRef.current = true;
      dialog.close();
    }
  }, [open]);

  function chooseCategory(id: string | null) {
    onCategoryChange(id);
    setCategoryMenuOpen(false);
    categoryButtonRef.current?.focus();
  }

  const selectedCategory = categories.find(
    (category) => category.id === categoryId,
  );

  return (
    <dialog
      ref={dialogRef}
      id="expense-filter-options"
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        if (categoryMenuOpen) {
          setCategoryMenuOpen(false);
          categoryButtonRef.current?.focus();
        } else {
          closeSheet();
        }
      }}
      onClose={() => {
        const expectedClose = expectedCloseEventRef.current;
        expectedCloseEventRef.current = false;
        setClosing(false);
        setCategoryMenuOpen(false);
        if (open && !expectedClose) onRequestClose();
      }}
      onAnimationEnd={(event) => {
        if (
          event.target === event.currentTarget &&
          event.animationName === "expense-filter-sheet-exit"
        ) {
          onRequestClose();
        }
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          closeSheet();
        } else if (
          categoryMenuOpen &&
          event.target instanceof Element &&
          !event.target.closest("[data-category-picker]")
        ) {
          setCategoryMenuOpen(false);
        }
      }}
      className={`expense-filter-sheet fixed inset-x-0 bottom-0 top-auto m-0 mx-auto w-full max-w-lg rounded-t-3xl border border-line bg-paper p-0 text-ink shadow-2xl backdrop:bg-overlay ${closing ? "is-closing" : ""}`}
    >
      <div className="expense-filter-sheet-content max-h-[82dvh] overflow-y-auto px-4 pt-4 pb-[calc(env(safe-area-inset-bottom,0px)+1rem)] sm:max-h-[min(80dvh,38rem)]">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 id={titleId} className="text-lg font-semibold">
              Filter expenses
            </h2>
            <p className="mt-0.5 text-xs text-ink-faint">
              Narrow the list by date and category.
            </p>
          </div>
          <button
            type="button"
            onClick={closeSheet}
            aria-label="Close filters"
            className="-mr-2 min-h-11 min-w-11 rounded-full text-xl text-ink-soft"
          >
            ×
          </button>
        </div>

        <div className="relative mt-4" data-category-picker>
          <p className="mb-2 text-xs font-medium text-ink-faint">Category</p>
          <button
            ref={categoryButtonRef}
            type="button"
            aria-label={`Category: ${selectedCategory?.name ?? "All categories"}`}
            aria-expanded={categoryMenuOpen}
            aria-controls="expense-category-options"
            onClick={() => setCategoryMenuOpen((isOpen) => !isOpen)}
            className="flex min-h-11 w-full items-center justify-between rounded-xl border border-line bg-paper-raised px-3 text-left text-sm text-ink focus:border-primary focus:outline-none"
          >
            <span>
              {selectedCategory?.icon ?? "◉"} {selectedCategory?.name ?? "All categories"}
            </span>
            <CategoryChevron expanded={categoryMenuOpen} />
          </button>
          {categoryMenuOpen ? (
            <div
              id="expense-category-options"
              aria-label="Choose a category"
              className="expense-category-menu mt-2 max-h-56 overflow-y-auto overscroll-contain rounded-2xl border border-line bg-paper-raised p-1.5 shadow-xl"
            >
              <CategoryOption
                label="All categories"
                icon="◉"
                selected={categoryId === null}
                animationIndex={0}
                onClick={() => chooseCategory(null)}
              />
              {categories.map((category, index) => (
                <CategoryOption
                  key={category.id}
                  label={category.name}
                  icon={category.icon}
                  selected={categoryId === category.id}
                  animationIndex={index + 1}
                  onClick={() => chooseCategory(category.id)}
                />
              ))}
            </div>
          ) : null}
        </div>

        <fieldset className="mt-5">
          <legend className="mb-2 text-xs font-medium text-ink-faint">
            Date range
          </legend>
          <div className="grid grid-cols-2 gap-2">
            <FilterChip
              label="This week"
              active={period === "week"}
              onClick={() => onPeriodChange("week")}
            />
            <FilterChip
              label="Last 14 days"
              active={period === "14days"}
              onClick={() => onPeriodChange("14days")}
            />
            <FilterChip
              label={month === karachiMonthKey(new Date()) ? "This month" : "Selected month"}
              active={period === "month"}
              onClick={() => onPeriodChange("month")}
            />
            <FilterChip
              label="Custom dates"
              active={period === "custom"}
              onClick={() => onPeriodChange("custom")}
            />
          </div>
        </fieldset>

        {period === "custom" ? (
          <div className="mt-4 grid grid-cols-2 gap-2">
            <label className="text-xs text-ink-faint">
              From
              <input
                aria-label="From date"
                type="date"
                value={customFrom}
                max={customTo}
                onChange={(event) => onCustomFromChange(event.target.value)}
                className="mt-1 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-2 text-sm text-ink"
              />
            </label>
            <label className="text-xs text-ink-faint">
              Through
              <input
                aria-label="Through date"
                type="date"
                value={customTo}
                min={customFrom}
                max={karachiDateInputValue(new Date())}
                onChange={(event) => onCustomToChange(event.target.value)}
                className="mt-1 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-2 text-sm text-ink"
              />
            </label>
          </div>
        ) : null}

        <div className="mt-5 flex items-center justify-between gap-3 border-t border-line pt-3">
          <button
            type="button"
            onClick={onClear}
            disabled={period === "month" && categoryId === null}
            className="min-h-11 px-2 text-sm font-medium text-ink-soft disabled:opacity-40"
          >
            Clear filters
          </button>
          <button
            type="button"
            onClick={closeSheet}
            className="min-h-11 rounded-xl bg-primary px-5 text-sm font-semibold text-paper"
          >
            Done
          </button>
        </div>
      </div>
    </dialog>
  );
}

function FilterIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M4 6h16M7 12h10m-7 6h4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
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
