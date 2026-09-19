import type { DebtEntry, Person } from "./types";

export const LEDGER_CACHE_FRESH_MS = 30_000;
const VERSION = 1;
const MAX_PERSON_ENTRY_LISTS = 20;

const storageKey = (userID: string) => `rakam.ledger.v${VERSION}.${userID}`;

export type CacheEntry<T> = { data: T; updatedAt: number };

export type LedgerCache = {
  version: 1;
  people: CacheEntry<Person[]> | null;
  entries: Record<string, CacheEntry<DebtEntry[]>>;
};

export function emptyLedgerCache(): LedgerCache {
  return { version: VERSION, people: null, entries: {} };
}

export function readLedgerCache(storage: Storage, userID: string): LedgerCache {
  const raw = storage.getItem(storageKey(userID));
  if (!raw) return emptyLedgerCache();
  try {
    const value: unknown = JSON.parse(raw);
    return isLedgerCache(value) ? value : emptyLedgerCache();
  } catch {
    return emptyLedgerCache();
  }
}

export function writeLedgerCache(
  storage: Storage,
  userID: string,
  cache: LedgerCache,
): void {
  storage.setItem(storageKey(userID), JSON.stringify(cache));
}

export function clearLedgerCache(storage: Storage, userID: string): void {
  storage.removeItem(storageKey(userID));
}

export function isLedgerFresh(
  entry: CacheEntry<unknown>,
  now = Date.now(),
): boolean {
  return now - entry.updatedAt < LEDGER_CACHE_FRESH_MS;
}

export function withEntryList(
  cache: LedgerCache,
  personID: string,
  entry: CacheEntry<DebtEntry[]>,
): LedgerCache {
  const entries = { ...cache.entries, [personID]: entry };
  const ids = Object.keys(entries);
  if (ids.length > MAX_PERSON_ENTRY_LISTS) {
    ids
      .sort((a, b) => entries[a].updatedAt - entries[b].updatedAt)
      .slice(0, ids.length - MAX_PERSON_ENTRY_LISTS)
      .forEach((oldest) => delete entries[oldest]);
  }
  return { ...cache, entries };
}

export function withoutPerson(
  cache: LedgerCache,
  personID: string,
): LedgerCache {
  const entries = { ...cache.entries };
  delete entries[personID];
  return {
    ...cache,
    people: cache.people
      ? {
          data: cache.people.data.filter((person) => person.id !== personID),
          updatedAt: Date.now(),
        }
      : null,
    entries,
  };
}

function isLedgerCache(value: unknown): value is LedgerCache {
  if (!value || typeof value !== "object") return false;
  const cache = value as Partial<LedgerCache>;
  if (cache.version !== VERSION) return false;
  if (cache.people !== null && !isEntry(cache.people, isPeople)) return false;
  if (!cache.entries || typeof cache.entries !== "object") return false;
  return Object.values(cache.entries).every((entry) =>
    isEntry(entry, isDebtEntries),
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

function isPeople(value: unknown): value is Person[] {
  return Array.isArray(value) && value.every(isPerson);
}

function isPerson(value: unknown): value is Person {
  if (!value || typeof value !== "object") return false;
  const person = value as Partial<Person>;
  return (
    typeof person.id === "string" &&
    typeof person.name === "string" &&
    (person.phone === null || typeof person.phone === "string") &&
    (person.notes === null || typeof person.notes === "string") &&
    isInteger(person.balance_paisa) &&
    typeof person.created_at === "string" &&
    typeof person.updated_at === "string"
  );
}

function isDebtEntries(value: unknown): value is DebtEntry[] {
  return Array.isArray(value) && value.every(isDebtEntry);
}

function isDebtEntry(value: unknown): value is DebtEntry {
  if (!value || typeof value !== "object") return false;
  const entry = value as Partial<DebtEntry>;
  return (
    typeof entry.id === "string" &&
    typeof entry.person_id === "string" &&
    (entry.direction === "i_owe" || entry.direction === "they_owe") &&
    isInteger(entry.amount_paisa) &&
    typeof entry.description === "string" &&
    typeof entry.incurred_at === "string" &&
    (entry.settled_at === null || typeof entry.settled_at === "string") &&
    typeof entry.created_at === "string" &&
    typeof entry.updated_at === "string"
  );
}

function isInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value);
}
