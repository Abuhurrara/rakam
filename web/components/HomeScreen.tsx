"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ApiError, getSummary } from "@/lib/api";
import {
  formatDayHeader,
  formatDueDate,
  formatMonthLabel,
  formatTime,
  karachiDayKey,
} from "@/lib/date";
import { budgetState, sortUpcomingBills } from "@/lib/home";
import { formatPaisa } from "@/lib/money";
import { isFresh } from "@/lib/finance-cache";
import type { Summary, Transaction } from "@/lib/types";
import { friendlyMessage } from "@/lib/useMutation";
import { useCategories } from "./CategoriesProvider";
import { useSaves } from "./SavesProvider";
import { useFinanceData } from "./FinanceDataProvider";

export function HomeScreen() {
  const { byId } = useCategories();
  const { revision } = useSaves();
  const financeData = useFinanceData();
  const [summary, setSummary] = useState<Summary | null>(
    () => financeData.readSummary()?.data ?? null,
  );
  const [loading, setLoading] = useState(
    () => financeData.readSummary() === null,
  );
  const [error, setError] = useState<ApiError | null>(null);
  const [reloadNonce, setReloadNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    const cached = financeData.readSummary();
    if (cached) setSummary(cached.data);
    if (reloadNonce === 0 && cached && isFresh(cached)) {
      setLoading(false);
      setError(null);
      return () => controller.abort();
    }
    setLoading(true);
    setError(null);

    getSummary(controller.signal)
      .then((next) => {
        if (!cancelled) {
          financeData.writeSummary(next);
          setSummary(next);
        }
      })
      .catch((err: unknown) => {
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
  }, [reloadNonce, revision, financeData]);

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") {
        setReloadNonce((n) => n + 1);
      }
    };
    window.addEventListener("online", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      window.removeEventListener("online", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, []);

  if (!summary && loading) return <HomeSkeleton />;
  if (!summary && error) {
    return (
      <HomeError
        message={friendlyMessage(error)}
        onRetry={() => setReloadNonce((n) => n + 1)}
      />
    );
  }
  if (!summary) return null;

  const upcoming = sortUpcomingBills(summary.upcoming_bills);

  return (
    <div className="px-4 pt-6">
      <header className="flex items-end justify-between gap-4">
        <div>
          <p className="text-label uppercase tracking-widest text-gold">
            Your ledger
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-ink">
            Rakam
          </h1>
        </div>
        <p className="pb-0.5 text-sm text-ink-soft">
          {formatMonthLabel(summary.month)}
        </p>
      </header>

      {loading ? (
        <p className="sr-only" role="status">
          Refreshing dashboard
        </p>
      ) : null}
      {error ? (
        <div
          role="alert"
          className="mt-4 flex items-center justify-between gap-3 rounded-xl border border-brick/30 bg-brick-tint px-4 py-3"
        >
          <p className="text-sm text-brick">{friendlyMessage(error)}</p>
          <button
            type="button"
            onClick={() => setReloadNonce((n) => n + 1)}
            className="min-h-11 shrink-0 px-2 text-sm font-semibold text-ink"
          >
            Retry
          </button>
        </div>
      ) : null}

      <MoneyOverview summary={summary} />
      <BudgetCard summary={summary} />
      <StreetCard summary={summary} />
      <UpcomingBills bills={upcoming} />
      <RecentTransactions
        transactions={summary.recent_transactions}
        byId={byId}
      />
    </div>
  );
}

function MoneyOverview({ summary }: { summary: Summary }) {
  return (
    <section className="mt-5 overflow-hidden rounded-3xl bg-primary px-5 py-6 text-primary-ink">
      <p className="text-label uppercase tracking-widest opacity-70">
        Spent this month
      </p>
      <p className="tabular mt-1 whitespace-nowrap text-[clamp(1.75rem,8.5vw,3rem)] leading-[1.05] font-semibold tracking-[-0.02em]">
        {formatPaisa(summary.expense_paisa)}
      </p>
      <div className="mt-5 grid grid-cols-2 gap-px overflow-hidden rounded-2xl bg-white/15">
        <div className="bg-primary px-3 py-3">
          <p className="text-xs opacity-70">Earned</p>
          <p className="tabular mt-0.5 text-money font-medium">
            {formatPaisa(summary.income_paisa)}
          </p>
        </div>
        <div className="bg-primary px-3 py-3">
          <p className="text-xs opacity-70">Net</p>
          <p className="tabular mt-0.5 text-money font-medium">
            {formatPaisa(summary.net_paisa, { signed: true })}
          </p>
        </div>
      </div>
    </section>
  );
}

function BudgetCard({ summary }: { summary: Summary }) {
  const state = budgetState(
    summary.budget_spent_paisa,
    summary.budget_limit_paisa,
  );
  const barTone = {
    primary: "bg-primary",
    gold: "bg-gold",
    brick: "bg-brick",
  }[state.tone];
  const textTone = {
    primary: "text-primary",
    gold: "text-gold",
    brick: "text-brick",
  }[state.tone];

  return (
    <section className="mt-4 rounded-2xl border border-line bg-paper-raised px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-ink">Monthly budget</h2>
          <p className="mt-0.5 text-xs text-ink-faint">
            {summary.days_remaining}{" "}
            {summary.days_remaining === 1 ? "day" : "days"} remaining
          </p>
        </div>
        {summary.budget_limit_paisa > 0 ? (
          <span className={`tabular text-sm font-semibold ${textTone}`}>
            {state.percent}%
          </span>
        ) : null}
      </div>

      {summary.budget_limit_paisa > 0 ? (
        <>
          <div
            role="progressbar"
            aria-label="Monthly budget used"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={state.visualPercent}
            aria-valuetext={`${state.percent}% used`}
            className="mt-4 h-2.5 overflow-hidden rounded-full bg-paper-sunken"
          >
            <div
              className={`h-full rounded-full ${barTone}`}
              style={{ width: `${state.visualPercent}%` }}
            />
          </div>
          <div className="mt-2 flex items-baseline justify-between gap-3">
            <span className="tabular text-sm text-ink">
              {formatPaisa(summary.budget_spent_paisa)} spent
            </span>
            <span className="tabular text-xs text-ink-faint">
              of {formatPaisa(summary.budget_limit_paisa)}
            </span>
          </div>
        </>
      ) : (
        <div className="mt-3 flex items-center justify-between gap-3 rounded-xl bg-paper-sunken px-3 py-3">
          <p className="text-sm text-ink-soft">No budget set for this month.</p>
          <Link
            href="/budget"
            className="flex min-h-11 shrink-0 items-center px-2 text-sm font-semibold text-primary"
          >
            Set budget
          </Link>
        </div>
      )}
    </section>
  );
}

function StreetCard({ summary }: { summary: Summary }) {
  const netTone = summary.net_owed_paisa >= 0 ? "text-primary" : "text-brick";
  return (
    <Link
      href="/ledger"
      className="mt-4 block min-h-11 rounded-2xl border border-line bg-paper-raised px-4 py-4 active:bg-paper-sunken"
    >
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-ink">
            Money on the street
          </h2>
          <p className="mt-0.5 text-xs text-ink-faint">Open your ledger</p>
        </div>
        <span className={`tabular text-money font-semibold ${netTone}`}>
          {formatPaisa(summary.net_owed_paisa, { signed: true })}
        </span>
      </div>
      <div className="mt-3 grid grid-cols-2 gap-3 border-t border-line pt-3">
        <div>
          <p className="text-xs text-ink-faint">Owed to you</p>
          <p className="tabular mt-0.5 text-sm font-medium text-primary">
            {formatPaisa(summary.owed_to_me_paisa)}
          </p>
        </div>
        <div className="text-right">
          <p className="text-xs text-ink-faint">You owe</p>
          <p className="tabular mt-0.5 text-sm font-medium text-brick">
            {formatPaisa(summary.i_owe_paisa)}
          </p>
        </div>
      </div>
    </Link>
  );
}

function UpcomingBills({ bills }: { bills: Summary["upcoming_bills"] }) {
  return (
    <section className="mt-7" aria-labelledby="upcoming-heading">
      <h2 id="upcoming-heading" className="text-base font-semibold text-ink">
        Due in the next 7 days
      </h2>
      {bills.length === 0 ? (
        <p className="mt-2 rounded-2xl border border-dashed border-line-strong px-4 py-5 text-sm text-ink-soft">
          No recurring bills due soon.
        </p>
      ) : (
        <ul className="mt-2 overflow-hidden rounded-2xl border border-line bg-paper-raised">
          {bills.map(({ bill, due_at }, index) => (
            <li
              key={`${bill.id}-${due_at}`}
              className={`flex min-h-14 items-center gap-3 px-4 py-3 ${
                index > 0 ? "border-t border-line" : ""
              }`}
            >
              <span
                aria-hidden="true"
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gold-tint text-gold"
              >
                ◷
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-ink">
                  {bill.name}
                </span>
                <span className="block text-xs text-ink-faint">
                  {formatDueDate(due_at)}
                </span>
              </span>
              <span className="tabular shrink-0 text-sm font-medium text-brick">
                {formatPaisa(bill.amount_paisa)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function RecentTransactions({
  transactions,
  byId,
}: {
  transactions: Transaction[];
  byId: Map<string, { name: string; icon: string }>;
}) {
  return (
    <section className="mt-7" aria-labelledby="recent-heading">
      <div className="flex items-center justify-between gap-3">
        <h2 id="recent-heading" className="text-base font-semibold text-ink">
          Recent transactions
        </h2>
        <Link
          href="/expenses"
          className="flex min-h-11 items-center px-2 text-sm font-semibold text-primary"
        >
          View expenses
        </Link>
      </div>
      {transactions.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-line-strong px-4 py-6 text-center">
          <p className="text-sm text-ink-soft">Your ledger is still empty.</p>
          <p className="mt-1 text-xs text-ink-faint">
            Tap + to add an expense.
          </p>
        </div>
      ) : (
        <ul className="overflow-hidden rounded-2xl border border-line bg-paper-raised">
          {transactions.map((transaction, index) => {
            const category = transaction.category_id
              ? byId.get(transaction.category_id)
              : undefined;
            const amount =
              transaction.kind === "expense"
                ? -transaction.amount_paisa
                : transaction.amount_paisa;
            return (
              <li
                key={transaction.id}
                className={`flex min-h-14 items-center gap-3 px-4 py-3 ${
                  index > 0 ? "border-t border-line" : ""
                }`}
              >
                <span
                  aria-hidden="true"
                  className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-paper-sunken"
                >
                  {category?.icon ?? "•"}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-ink">
                    {transaction.description ||
                      category?.name ||
                      "Uncategorised"}
                  </span>
                  <span className="block truncate text-xs text-ink-faint">
                    {transaction.description && category
                      ? `${category.name} · `
                      : ""}
                    {formatDayHeader(karachiDayKey(transaction.occurred_at))} ·{" "}
                    {formatTime(transaction.occurred_at)}
                  </span>
                </span>
                <span
                  className={`tabular shrink-0 text-sm font-medium ${
                    transaction.kind === "expense"
                      ? "text-brick"
                      : "text-primary"
                  }`}
                >
                  {formatPaisa(amount, { signed: true })}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function HomeSkeleton() {
  return (
    <div className="px-4 pt-6" aria-busy="true" aria-label="Loading dashboard">
      <span className="sr-only" role="status">
        Loading dashboard
      </span>
      <div aria-hidden="true" className="animate-pulse">
        <div className="flex items-end justify-between">
          <div className="space-y-2">
            <div className="h-2.5 w-20 rounded bg-paper-sunken" />
            <div className="h-7 w-24 rounded bg-paper-sunken" />
          </div>
          <div className="h-4 w-28 rounded bg-paper-sunken" />
        </div>
        <div className="mt-5 h-48 rounded-3xl bg-primary-tint" />
        <div className="mt-4 h-32 rounded-2xl bg-paper-raised" />
        <div className="mt-4 h-32 rounded-2xl bg-paper-raised" />
        <div className="mt-7 h-5 w-40 rounded bg-paper-sunken" />
        <div className="mt-2 h-20 rounded-2xl bg-paper-raised" />
        <div className="mt-7 h-5 w-44 rounded bg-paper-sunken" />
        <div className="mt-2 h-48 rounded-2xl bg-paper-raised" />
      </div>
    </div>
  );
}

function HomeError({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div className="px-4 pt-6">
      <h1 className="text-2xl font-semibold text-ink">Rakam</h1>
      <div
        role="alert"
        className="mt-5 rounded-2xl border border-brick/30 bg-brick-tint px-5 py-8 text-center"
      >
        <p className="text-sm font-semibold text-brick">
          Couldn&apos;t load Home
        </p>
        <p className="mt-1 text-xs text-ink-soft">{message}</p>
        <button
          type="button"
          onClick={onRetry}
          className="mt-4 min-h-11 rounded-full border border-line-strong px-5 text-sm font-semibold text-ink"
        >
          Try again
        </button>
      </div>
    </div>
  );
}
