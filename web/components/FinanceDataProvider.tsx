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
  clearFinanceCache,
  emptyFinanceCache,
  invalidateFinanceCache,
  readFinanceCache,
  withTransactionEntry,
  writeFinanceCache,
  type CacheEntry,
  type FinanceCache,
} from "@/lib/finance-cache";
import type { Summary, TransactionList } from "@/lib/types";

type FinanceDataValue = {
  readSummary: () => CacheEntry<Summary> | null;
  writeSummary: (data: Summary) => void;
  readTransactions: (key: string) => CacheEntry<TransactionList> | null;
  writeTransactions: (key: string, data: TransactionList) => void;
  invalidate: () => void;
  clear: () => void;
};

const FinanceDataContext = createContext<FinanceDataValue | null>(null);

export function FinanceDataProvider({
  userID,
  children,
}: {
  userID: string | null;
  children: ReactNode;
}) {
  const cache = useRef<FinanceCache>(
    userID ? readFinanceCache(localStorage, userID) : emptyFinanceCache(),
  );

  const persist = useCallback(
    (next: FinanceCache) => {
      cache.current = next;
      if (!userID) return;
      try {
        writeFinanceCache(localStorage, userID, next);
      } catch {
        // The in-memory cache still removes repeat waits for this app session.
      }
    },
    [userID],
  );

  const readSummary = useCallback(() => cache.current.summary, []);
  const writeSummary = useCallback(
    (data: Summary) =>
      persist({ ...cache.current, summary: { data, updatedAt: Date.now() } }),
    [persist],
  );
  const readTransactions = useCallback(
    (key: string) => cache.current.transactions[key] ?? null,
    [],
  );
  const writeTransactions = useCallback(
    (key: string, data: TransactionList) =>
      persist(
        withTransactionEntry(cache.current, key, {
          data,
          updatedAt: Date.now(),
        }),
      ),
    [persist],
  );
  const invalidate = useCallback(
    () => persist(invalidateFinanceCache(cache.current)),
    [persist],
  );
  const clear = useCallback(() => {
    cache.current = emptyFinanceCache();
    if (!userID) return;
    try {
      clearFinanceCache(localStorage, userID);
    } catch {
      // A full navigation still clears memory and the auth gate hides data.
    }
  }, [userID]);

  const value = useMemo<FinanceDataValue>(
    () => ({
      readSummary,
      writeSummary,
      readTransactions,
      writeTransactions,
      invalidate,
      clear,
    }),
    [
      readSummary,
      writeSummary,
      readTransactions,
      writeTransactions,
      invalidate,
      clear,
    ],
  );

  return (
    <FinanceDataContext.Provider value={value}>
      {children}
    </FinanceDataContext.Provider>
  );
}

export function useFinanceData(): FinanceDataValue {
  const value = useContext(FinanceDataContext);
  if (!value) {
    throw new Error("useFinanceData must be used inside <FinanceDataProvider>");
  }
  return value;
}
