"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import {
  clearLedgerCache,
  emptyLedgerCache,
  readLedgerCache,
  withEntryList,
  withoutPerson,
  writeLedgerCache,
  type CacheEntry,
  type LedgerCache,
} from "@/lib/ledger-cache";
import type { DebtEntry, Person } from "@/lib/types";

type LedgerDataValue = {
  readPeople: () => CacheEntry<Person[]> | null;
  writePeople: (people: Person[]) => void;
  upsertPerson: (person: Person) => void;
  removePerson: (personID: string) => void;
  readEntries: (personID: string) => CacheEntry<DebtEntry[]> | null;
  writeEntries: (personID: string, entries: DebtEntry[]) => void;
  clear: () => void;
};

const LedgerDataContext = createContext<LedgerDataValue | null>(null);

/**
 * Ledger data follows the same account-scoped stale-while-revalidate model as
 * Home and Expenses. The provider survives tab navigation, while localStorage
 * makes a recently visited ledger paint immediately after an app restart.
 */
export function LedgerDataProvider({
  userID,
  children,
}: {
  userID: string | null;
  children: ReactNode;
}) {
  const cache = useRef<LedgerCache>(
    userID ? readLedgerCache(localStorage, userID) : emptyLedgerCache(),
  );

  const persist = useCallback(
    (next: LedgerCache) => {
      cache.current = next;
      if (!userID) return;
      try {
        writeLedgerCache(localStorage, userID, next);
      } catch {
        // The in-memory cache still removes repeat waits for this app session.
      }
    },
    [userID],
  );

  const readPeople = useCallback(() => cache.current.people, []);
  const writePeople = useCallback(
    (people: Person[]) =>
      persist({ ...cache.current, people: { data: people, updatedAt: Date.now() } }),
    [persist],
  );
  const upsertPerson = useCallback(
    (person: Person) => {
      const cached = cache.current.people;
      if (!cached) return;
      const hasPerson = cached.data.some((current) => current.id === person.id);
      persist({
        ...cache.current,
        people: {
          data: hasPerson
            ? cached.data.map((current) =>
                current.id === person.id ? person : current,
              )
            : [...cached.data, person],
          updatedAt: Date.now(),
        },
      });
    },
    [persist],
  );
  const removePerson = useCallback(
    (personID: string) => persist(withoutPerson(cache.current, personID)),
    [persist],
  );
  const readEntries = useCallback(
    (personID: string) => cache.current.entries[personID] ?? null,
    [],
  );
  const writeEntries = useCallback(
    (personID: string, entries: DebtEntry[]) =>
      persist(
        withEntryList(cache.current, personID, {
          data: entries,
          updatedAt: Date.now(),
        }),
      ),
    [persist],
  );
  const clear = useCallback(() => {
    cache.current = emptyLedgerCache();
    if (!userID) return;
    try {
      clearLedgerCache(localStorage, userID);
    } catch {
      // A full navigation still clears memory and the auth gate hides data.
    }
  }, [userID]);

  const value = useMemo<LedgerDataValue>(
    () => ({
      readPeople,
      writePeople,
      upsertPerson,
      removePerson,
      readEntries,
      writeEntries,
      clear,
    }),
    [
      readPeople,
      writePeople,
      upsertPerson,
      removePerson,
      readEntries,
      writeEntries,
      clear,
    ],
  );

  return (
    <LedgerDataContext.Provider value={value}>
      {children}
    </LedgerDataContext.Provider>
  );
}

export function useLedgerData(): LedgerDataValue {
  const value = useContext(LedgerDataContext);
  if (!value) {
    throw new Error("useLedgerData must be used inside <LedgerDataProvider>");
  }
  return value;
}
