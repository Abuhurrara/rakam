import type { UpcomingBill } from "./types";

export type BudgetState = {
  percent: number;
  visualPercent: number;
  tone: "primary" | "gold" | "brick";
};

/** Derive display-only progress without changing either paisa value. */
export function budgetState(
  spentPaisa: number,
  limitPaisa: number,
): BudgetState {
  if (limitPaisa <= 0) {
    return { percent: 0, visualPercent: 0, tone: "primary" };
  }

  const percent = Math.round((spentPaisa * 100) / limitPaisa);
  return {
    percent,
    visualPercent: Math.min(100, Math.max(0, percent)),
    tone: percent >= 100 ? "brick" : percent >= 80 ? "gold" : "primary",
  };
}

/** The repository orders bills by configured day, which can straddle months. */
export function sortUpcomingBills(
  bills: readonly UpcomingBill[],
): UpcomingBill[] {
  return [...bills].sort(
    (a, b) =>
      a.due_at.localeCompare(b.due_at) ||
      a.bill.name.localeCompare(b.bill.name),
  );
}
