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
  active: Category[];
  expense: Category[];
  byId: Map<string, Category>;
  loading: boolean;
  error: ApiError | null;
  reload: () => void;
  saveLocal: (category: Category) => void;
  archiveLocal: (id: string) => void;
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
  const saveLocal = useCallback((category: Category) => {
    setAll((current) => {
      const found = current.some((item) => item.id === category.id);
      const next = found
        ? current.map((item) => item.id === category.id ? category : item)
        : [...current, category];
      return next.sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));
    });
  }, []);
  const archiveLocal = useCallback((id: string) => {
    setAll((current) => current.map((category) =>
      category.id === id ? { ...category, is_archived: true } : category,
    ));
  }, []);

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
    () => {
      const active = all.filter((category) => !category.is_archived);
      return {
        all,
        active,
        expense: active.filter((category) => category.kind === "expense"),
        byId: new Map(all.map((category) => [category.id, category])),
        loading,
        error,
        reload,
        saveLocal,
        archiveLocal,
      };
    },
    [all, loading, error, reload, saveLocal, archiveLocal],
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
