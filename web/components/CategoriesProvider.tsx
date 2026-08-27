"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { ApiError, listCategories } from "@/lib/api";
import type { Category } from "@/lib/types";

/**
 * Categories are fetched once for the whole session and held here.
 *
 * This is what lets the add sheet open with its chips already on screen: no
 * network call sits between tapping "+" and being able to type. That single
 * fact is most of the 5-second budget.
 */

type CategoriesValue = {
  all: Category[];
  expense: Category[];
  byId: Map<string, Category>;
  loading: boolean;
  error: ApiError | null;
  reload: () => void;
};

const CategoriesContext = createContext<CategoriesValue | null>(null);

export function CategoriesProvider({
  enabled,
  children,
}: {
  /** Stays false until the session is confirmed, so we never fetch data
      against a session we have not verified. */
  enabled: boolean;
  children: ReactNode;
}) {
  const [all, setAll] = useState<Category[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setLoading(true);
    setError(null);

    listCategories()
      .then((cats) => {
        if (!cancelled) setAll(cats);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err : new ApiError("Failed"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [enabled, nonce]);

  const value = useMemo<CategoriesValue>(
    () => ({
      all,
      expense: all.filter((c) => c.kind === "expense"),
      byId: new Map(all.map((c) => [c.id, c])),
      loading,
      error,
      reload,
    }),
    [all, loading, error, reload],
  );

  return (
    <CategoriesContext.Provider value={value}>
      {children}
    </CategoriesContext.Provider>
  );
}

export function useCategories(): CategoriesValue {
  const ctx = useContext(CategoriesContext);
  if (!ctx) {
    throw new Error("useCategories must be used inside <CategoriesProvider>");
  }
  return ctx;
}
