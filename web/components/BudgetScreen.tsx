"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { ApiError, deleteBudget, listBudgets, upsertBudget } from "@/lib/api";
import { formatMonthLabel, isFutureMonth, karachiMonthKey, shiftMonthKey } from "@/lib/date";
import { isFresh } from "@/lib/finance-cache";
import { budgetState } from "@/lib/home";
import { formatPaisa, isValidAmountInput, paisaToInputString, sumPaisa } from "@/lib/money";
import type { Budget, BudgetInput, BudgetWithSpent } from "@/lib/types";
import { friendlyMessage, useMutation } from "@/lib/useMutation";
import { useFinanceData } from "./FinanceDataProvider";
import { useSaves } from "./SavesProvider";
import { Spinner } from "./Spinner";

export function BudgetScreen() {
  const financeData = useFinanceData();
  const { revision } = useSaves();
  const [month, setMonth] = useState(() => karachiMonthKey(new Date()));
  const [result, setResult] = useState<{ month: string; rows: BudgetWithSpent[] } | null>(() => {
    const current = karachiMonthKey(new Date());
    const cached = financeData.readBudgets(current);
    return cached ? { month: current, rows: cached.data } : null;
  });
  const [loading, setLoading] = useState(() => financeData.readBudgets(karachiMonthKey(new Date())) === null);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<{ month: string; value: ApiError } | null>(null);
  const [reloadNonce, setReloadNonce] = useState(0);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [amount, setAmount] = useState("");
  const [removeArmed, setRemoveArmed] = useState(false);
  const mutationVersion = useRef(0);
  const save = useMutation<BudgetInput, Budget>(upsertBudget);
  const remove = useMutation<string, null>(deleteBudget);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    const version = mutationVersion.current;
    const cached = financeData.readBudgets(month);
    if (cached) setResult({ month, rows: cached.data });
    if (cached && isFresh(cached)) {
      setLoading(false);
      setRefreshing(false);
      setError(null);
      return () => controller.abort();
    }
    setLoading(cached === null);
    setRefreshing(cached !== null);
    setError(null);

    listBudgets(month, controller.signal)
      .then((rows) => {
        if (cancelled || version !== mutationVersion.current) return;
        financeData.writeBudgets(month, rows);
        setResult({ month, rows });
      })
      .catch((err: unknown) => {
        if (cancelled || version !== mutationVersion.current) return;
        setError({ month, value: err instanceof ApiError ? err : new ApiError("Failed to load budgets.") });
      })
      .finally(() => {
        if (cancelled || version !== mutationVersion.current) return;
        setLoading(false);
        setRefreshing(false);
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [month, reloadNonce, revision, financeData]);

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

  const cachedForMonth = financeData.readBudgets(month);
  const rows = result?.month === month ? result.rows : (cachedForMonth?.data ?? null);
  const withLimits = rows?.filter((row) => row.budget !== null) ?? [];
  const withoutLimits = rows?.filter((row) => row.budget === null) ?? [];
  const currentError = error?.month === month ? error.value : null;
  const busy = save.status === "pending" || remove.status === "pending";

  const startEdit = (row: BudgetWithSpent) => {
    setEditingID(row.category.id);
    setAmount(row.budget ? paisaToInputString(row.budget.limit_paisa) : "");
    setRemoveArmed(false);
    save.reset();
    remove.reset();
  };
  const applyChange = (categoryID: string, budget: Budget | null) => {
    mutationVersion.current++;
    financeData.recordBudgetChange(month, categoryID, budget);
    const updated = financeData.readBudgets(month);
    if (updated) setResult({ month, rows: updated.data });
    setEditingID(null);
    setRemoveArmed(false);
    // The changed row is already visible; confirm spending in the background.
    setReloadNonce((n) => n + 1);
  };
  const submit = async (event: FormEvent<HTMLFormElement>, categoryID: string) => {
    event.preventDefault();
    if (busy || !isValidAmountInput(amount)) return;
    const response = await save.run({ category_id: categoryID, month, limit: amount.trim() });
    if (response.ok) applyChange(categoryID, response.data);
  };
  const confirmRemove = async (budget: Budget) => {
    if (busy) return;
    const response = await remove.run(budget.id);
    if (response.ok) applyChange(budget.category_id, null);
  };

  return (
    <div className="px-4 pt-4">
      <div className="flex items-center justify-between">
        <button type="button" aria-label="Previous month" disabled={busy}
          onClick={() => { setEditingID(null); setMonth(shiftMonthKey(month, -1)); }}
          className="flex h-11 w-11 items-center justify-center rounded-full text-xl text-ink-soft active:bg-paper-sunken disabled:opacity-40">‹</button>
        <h1 className="text-base font-semibold text-ink">{formatMonthLabel(month)}</h1>
        <button type="button" aria-label="Next month"
          disabled={busy || isFutureMonth(shiftMonthKey(month, 1))}
          onClick={() => { setEditingID(null); setMonth(shiftMonthKey(month, 1)); }}
          className="flex h-11 w-11 items-center justify-center rounded-full text-xl text-ink-soft active:bg-paper-sunken disabled:opacity-25">›</button>
      </div>
      <header className="mt-4">
        <p className="text-label uppercase tracking-widest text-ink-faint">Plan your spending</p>
        <h2 className="mt-1 text-2xl font-semibold tracking-tight">Budget</h2>
        <p className="mt-1 text-sm text-ink-soft">Set a limit for each expense category.</p>
      </header>

      {rows ? <BudgetOverview rows={rows} /> : null}
      {refreshing ? <p role="status" className="sr-only">Refreshing budgets</p> : null}
      {currentError && rows ? (
        <div role="alert" className="mt-4 flex items-center justify-between gap-3 rounded-xl bg-brick-tint px-3 py-2.5">
          <p className="text-sm text-brick">{friendlyMessage(currentError)} Showing saved data.</p>
          <button type="button" onClick={() => setReloadNonce((n) => n + 1)} className="min-h-11 px-2 text-sm font-semibold text-ink">Retry</button>
        </div>
      ) : null}
      {!rows && (loading || !currentError) ? <div role="status" className="mt-8 flex items-center justify-center gap-2 text-sm text-ink-soft"><Spinner /> Loading budgets…</div> : null}
      {!rows && currentError ? (
        <div role="alert" className="mt-8 rounded-2xl border border-brick/30 bg-brick-tint px-5 py-6 text-center">
          <p className="font-medium text-brick">Couldn&apos;t load Budget</p>
          <p className="mt-1 text-sm text-ink-soft">{friendlyMessage(currentError)}</p>
          <button type="button" onClick={() => setReloadNonce((n) => n + 1)} className="mt-4 min-h-11 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink">Retry</button>
        </div>
      ) : null}
      {rows?.length === 0 ? <p className="mt-8 rounded-2xl border border-dashed border-line-strong p-6 text-center text-sm text-ink-soft">No expense categories yet.</p> : null}
      {withLimits.length > 0 ? <BudgetGroup title="Your limits" rows={withLimits} editingID={editingID} amount={amount}
        removeArmed={removeArmed} busy={busy} error={save.error ?? remove.error} onEdit={startEdit}
        onAmountChange={setAmount} onCancel={() => setEditingID(null)} onSubmit={submit}
        onArmRemove={() => setRemoveArmed(true)} onRemove={confirmRemove} /> : null}
      {withoutLimits.length > 0 ? <BudgetGroup title="Without a limit" rows={withoutLimits} editingID={editingID} amount={amount}
        removeArmed={removeArmed} busy={busy} error={save.error ?? remove.error} onEdit={startEdit}
        onAmountChange={setAmount} onCancel={() => setEditingID(null)} onSubmit={submit}
        onArmRemove={() => setRemoveArmed(true)} onRemove={confirmRemove} /> : null}
    </div>
  );
}

function BudgetOverview({ rows }: { rows: BudgetWithSpent[] }) {
  const limit = sumPaisa(rows.map((row) => row.budget?.limit_paisa ?? 0));
  const spent = sumPaisa(rows.filter((row) => row.budget !== null).map((row) => row.spent_paisa));
  const unbudgetedSpent = sumPaisa(rows.filter((row) => row.budget === null).map((row) => row.spent_paisa));
  const state = budgetState(spent, limit);
  return (
    <section className="mt-5 rounded-2xl border border-line bg-paper-raised px-4 py-4" aria-label="Budget overview">
      <p className="text-label uppercase tracking-widest text-ink-faint">{limit > 0 ? "Budgeted spending" : "Spending in listed categories"}</p>
      <p className="tabular mt-1 text-money-lg font-semibold">{formatPaisa(limit > 0 ? spent : unbudgetedSpent)}</p>
      <p className="mt-1 text-sm text-ink-soft">{limit ? `spent of ${formatPaisa(limit)}` : "No limits set for this month"}</p>
      {limit > 0 && unbudgetedSpent > 0 ? <p className="mt-1 text-xs text-ink-faint">{formatPaisa(unbudgetedSpent)} spent in categories without limits</p> : null}
      {limit > 0 ? <Progress spent={spent} limit={limit} label="All budget limits used"
        percent={state.percent} overPaisa={state.overPaisa} width={state.visualPercent} tone={state.tone} /> : null}
    </section>
  );
}

type GroupProps = {
  title: string;
  rows: BudgetWithSpent[];
  editingID: string | null;
  amount: string;
  removeArmed: boolean;
  busy: boolean;
  error: ApiError | null;
  onEdit: (row: BudgetWithSpent) => void;
  onAmountChange: (value: string) => void;
  onCancel: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>, categoryID: string) => void;
  onArmRemove: () => void;
  onRemove: (budget: Budget) => void;
};

function BudgetGroup(props: GroupProps) {
  return (
    <section className="mt-6">
      <h2 className="mb-2 text-sm font-semibold text-ink-soft">{props.title}</h2>
      <ul className="space-y-2">
        {props.rows.map((row) => {
          const state = row.budget ? budgetState(row.spent_paisa, row.budget.limit_paisa) : null;
          return (
            <li key={row.category.id} className="rounded-2xl border border-line bg-paper-raised px-4 py-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="font-medium text-ink"><span aria-hidden="true">{row.category.icon} </span>{row.category.name}</p>
                  <p className="tabular mt-1 text-sm text-ink-soft">
                    {formatPaisa(row.spent_paisa)} spent{row.budget ? ` of ${formatPaisa(row.budget.limit_paisa)}` : ""}
                  </p>
                </div>
                {props.editingID !== row.category.id ? (
                  <button type="button" disabled={props.busy} onClick={() => props.onEdit(row)}
                    className="min-h-11 shrink-0 px-2 text-sm font-semibold text-primary disabled:opacity-40">
                    {row.budget ? "Edit limit" : "Set limit"}
                  </button>
                ) : null}
              </div>
              {row.budget && state ? <Progress spent={row.spent_paisa} limit={row.budget.limit_paisa}
                label={`${row.category.name} budget used`} percent={state.percent} overPaisa={state.overPaisa} width={state.visualPercent} tone={state.tone} /> : null}
              {props.editingID === row.category.id ? (
                <form onSubmit={(event) => props.onSubmit(event, row.category.id)} className="mt-4 border-t border-line pt-4">
                  <label htmlFor={`budget-${row.category.id}`} className="block text-sm font-medium text-ink">Monthly limit in rupees</label>
                  <input id={`budget-${row.category.id}`} autoFocus inputMode="decimal" type="text" value={props.amount}
                    onChange={(event) => props.onAmountChange(event.target.value)} placeholder="0.00"
                    className="mt-1.5 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3 text-ink focus:border-primary focus:outline-none" />
                  {props.error ? <p role="alert" className="mt-2 text-sm text-brick">{friendlyMessage(props.error)}</p> : null}
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <button type="submit" disabled={props.busy || !isValidAmountInput(props.amount)}
                      className="min-h-11 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink disabled:opacity-40">
                      {props.busy ? "Saving…" : "Save limit"}
                    </button>
                    <button type="button" disabled={props.busy} onClick={props.onCancel}
                      className="min-h-11 rounded-xl px-3 text-sm font-medium text-ink-soft disabled:opacity-40">Cancel</button>
                    {row.budget ? props.removeArmed ? (
                      <button type="button" disabled={props.busy} onClick={() => props.onRemove(row.budget!)}
                        className="min-h-11 rounded-xl px-3 text-sm font-semibold text-brick disabled:opacity-40">Confirm remove</button>
                    ) : (
                      <button type="button" disabled={props.busy} onClick={props.onArmRemove}
                        className="min-h-11 rounded-xl px-3 text-sm font-medium text-brick disabled:opacity-40">Remove limit</button>
                    ) : null}
                  </div>
                </form>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function Progress({ spent, limit, percent, overPaisa, width, tone, label }: {
  spent: number;
  limit: number;
  percent: number;
  overPaisa: number;
  width: number;
  tone: "primary" | "gold" | "brick";
  label: string;
}) {
  const barTone = { primary: "bg-primary", gold: "bg-gold", brick: "bg-brick" }[tone];
  const textTone = { primary: "text-primary", gold: "text-gold", brick: "text-brick" }[tone];
  return (
    <div className="mt-3">
      <div className="flex justify-end"><span className={`tabular text-xs font-semibold ${textTone}`}>{overPaisa > 0 ? `${formatPaisa(overPaisa)} over` : `${percent}%`}</span></div>
      <div role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100}
        aria-valuenow={width} aria-valuetext={overPaisa > 0
          ? `${formatPaisa(overPaisa)} over limit: ${formatPaisa(spent)} of ${formatPaisa(limit)}`
          : `${percent}% used: ${formatPaisa(spent)} of ${formatPaisa(limit)}`}
        className="mt-1 h-2 overflow-hidden rounded-full bg-paper-sunken">
        <div className={`h-full rounded-full ${barTone}`} style={{ width: `${width}%` }} />
      </div>
    </div>
  );
}
