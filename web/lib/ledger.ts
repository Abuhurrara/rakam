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
