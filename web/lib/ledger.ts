import type { DebtDirection, DebtEntry, Kind } from "./types";

export type BalanceTone = "owed" | "owing" | "settled";

export function balanceMeta(balancePaisa: number): {
  label: string;
  tone: BalanceTone;
} {
  if (balancePaisa > 0) return { label: "Owes you", tone: "owed" };
  if (balancePaisa < 0) return { label: "You owe", tone: "owing" };
  return { label: "Settled up", tone: "settled" };
}

export function splitDebtEntries(entries: readonly DebtEntry[]): {
  unsettled: DebtEntry[];
  settled: DebtEntry[];
} {
  const unsettled: DebtEntry[] = [];
  const settled: DebtEntry[] = [];
  for (const entry of entries) {
    if (entry.settled_at === null) unsettled.push(entry);
    else settled.push(entry);
  }
  return { unsettled, settled };
}

/** The contribution an entry makes to a person's current net balance. */
export function debtEntryBalancePaisa(entry: DebtEntry): number {
  if (entry.settled_at !== null) return 0;
  return entry.direction === "they_owe"
    ? entry.amount_paisa
    : -entry.amount_paisa;
}

export type LedgerTotalsDelta = {
  owedToMePaisa: number;
  iOwePaisa: number;
};

export function debtEntryTotals(entry: DebtEntry): LedgerTotalsDelta {
  if (entry.settled_at !== null) {
    return { owedToMePaisa: 0, iOwePaisa: 0 };
  }
  return entry.direction === "they_owe"
    ? { owedToMePaisa: entry.amount_paisa, iOwePaisa: 0 }
    : { owedToMePaisa: 0, iOwePaisa: entry.amount_paisa };
}

export function subtractLedgerTotals(
  totals: LedgerTotalsDelta,
): LedgerTotalsDelta {
  return {
    owedToMePaisa: -totals.owedToMePaisa,
    iOwePaisa: -totals.iOwePaisa,
  };
}

/**
 * Replaces server-confirmed debt entries in a list and reports the exact
 * balance delta. This lets the screen paint a settlement immediately, then
 * reconcile quietly in the background.
 */
export function replaceDebtEntries(
  entries: readonly DebtEntry[],
  changed: readonly DebtEntry[],
): {
  entries: DebtEntry[];
  balanceDelta: number;
  totalsDelta: LedgerTotalsDelta;
} {
  const replacements = new Map(changed.map((entry) => [entry.id, entry]));
  let balanceDelta = 0;
  const totalsDelta: LedgerTotalsDelta = { owedToMePaisa: 0, iOwePaisa: 0 };
  const next = entries.map((entry) => {
    const replacement = replacements.get(entry.id);
    if (!replacement) return entry;
    balanceDelta +=
      debtEntryBalancePaisa(replacement) - debtEntryBalancePaisa(entry);
    const before = debtEntryTotals(entry);
    const after = debtEntryTotals(replacement);
    totalsDelta.owedToMePaisa += after.owedToMePaisa - before.owedToMePaisa;
    totalsDelta.iOwePaisa += after.iOwePaisa - before.iOwePaisa;
    return replacement;
  });
  return { entries: next, balanceDelta, totalsDelta };
}

export function categoryKindForDirection(direction: DebtDirection): Kind {
  return direction === "they_owe" ? "income" : "expense";
}

/** Returns null when a batch has both directions, so one category is unsafe. */
export function commonDirection(
  entries: readonly DebtEntry[],
): DebtDirection | null {
  const first = entries[0]?.direction;
  if (!first) return null;
  return entries.every((entry) => entry.direction === first) ? first : null;
}

export function directionLabel(direction: DebtDirection): string {
  return direction === "they_owe" ? "They owe you" : "You owe them";
}
