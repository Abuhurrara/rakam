"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  ApiError,
  createTransaction,
  deleteTransaction,
  updateTransaction,
} from "@/lib/api";
import { friendlyMessage } from "@/lib/useMutation";
import { mark, reportAddFlow } from "@/lib/perf";
import type { Transaction, TransactionInput } from "@/lib/types";
import { useToast } from "./Toast";

/**
 * Every transaction write goes through here, so the three mutation states are
 * handled in one place instead of being re-remembered per screen.
 *
 *   pending — a create sits in `pending` and the list renders it as a dimmed
 *             row; an update marks its row id in `updating`
 *   success — the entry leaves `pending`/`updating` and the real transaction
 *             is handed to the list, so the dimmed row turns solid. That
 *             solidifying IS the success signal — there is deliberately no
 *             success toast, because a toast plus the row would be two
 *             signals for one event on the app's most frequent action.
 *   failure — the entry leaves `pending` (the list must never show a
 *             transaction that is not saved) and a loud, persistent toast
 *             appears with a Retry that re-sends the identical payload.
 */

export type PendingSave = { key: string; input: TransactionInput };

type SavesValue = {
  pending: PendingSave[];
  updating: ReadonlySet<string>;
  save: (input: TransactionInput) => void;
  update: (id: string, input: TransactionInput) => void;
  remove: (t: Transaction) => void;
  subscribeCreated: (cb: (t: Transaction) => void) => () => void;
  subscribeUpdated: (cb: (t: Transaction) => void) => () => void;
  subscribeDeleted: (cb: (id: string) => void) => () => void;
  /** A delete that failed and was put back — restores the row in the list. */
  subscribeRestored: (cb: (t: Transaction) => void) => () => void;
};

const SavesContext = createContext<SavesValue | null>(null);

export function SavesProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<PendingSave[]>([]);
  const [updating, setUpdating] = useState<ReadonlySet<string>>(new Set());
  const created = useRef(new Set<(t: Transaction) => void>());
  const updated = useRef(new Set<(t: Transaction) => void>());
  const deleted = useRef(new Set<(id: string) => void>());
  const restored = useRef(new Set<(t: Transaction) => void>());
  const seq = useRef(0);
  const toast = useToast();

  const save = useCallback(
    (input: TransactionInput) => {
      const key = `p${++seq.current}`;
      setPending((p) => [...p, { key, input }]);

      createTransaction(input)
        .then((t) => {
          mark("saveDone");
          reportAddFlow("saved");
          setPending((p) => p.filter((x) => x.key !== key));
          created.current.forEach((cb) => cb(t));
        })
        .catch((err: unknown) => {
          mark("saveDone");
          reportAddFlow("failed");
          setPending((p) => p.filter((x) => x.key !== key));
          const e =
            err instanceof ApiError ? err : new ApiError("Could not save.");
          // A 401 is already navigating to /login; a toast on the way out
          // would just be noise.
          if (e.status === 401) return;
          toast.show({
            kind: "error",
            message: `Couldn't save Rs ${input.amount}`,
            detail: friendlyMessage(e),
            action: { label: "Retry", onClick: () => saveRef.current(input) },
          });
        });
    },
    [toast],
  );

  const update = useCallback(
    (id: string, input: TransactionInput) => {
      setUpdating((s) => new Set(s).add(id));

      const clear = () =>
        setUpdating((s) => {
          const next = new Set(s);
          next.delete(id);
          return next;
        });

      updateTransaction(id, input)
        .then((t) => {
          clear();
          updated.current.forEach((cb) => cb(t));
        })
        .catch((err: unknown) => {
          clear();
          const e =
            err instanceof ApiError ? err : new ApiError("Could not save.");
          if (e.status === 401) return;
          toast.show({
            kind: "error",
            message: "Couldn't save your change",
            detail: friendlyMessage(e),
            action: {
              label: "Retry",
              onClick: () => updateRef.current(id, input),
            },
          });
        });
    },
    [toast],
  );

  /**
   * Optimistic delete: the row goes immediately, and comes back if the
   * server refuses. A 404 counts as success — it is already gone.
   */
  const remove = useCallback(
    (t: Transaction) => {
      deleted.current.forEach((cb) => cb(t.id));

      deleteTransaction(t.id).catch((err: unknown) => {
        const e =
          err instanceof ApiError ? err : new ApiError("Could not delete.");
        if (e.status === 401) return;
        if (e.status === 404) return;
        restored.current.forEach((cb) => cb(t));
        toast.show({
          kind: "error",
          message: "Couldn't delete that",
          detail: friendlyMessage(e),
          action: { label: "Retry", onClick: () => removeRef.current(t) },
        });
      });
    },
    [toast],
  );

  // Let the Retry actions re-enter without the callbacks depending on
  // themselves.
  const saveRef = useRef(save);
  const updateRef = useRef(update);
  const removeRef = useRef(remove);
  useEffect(() => {
    saveRef.current = save;
    updateRef.current = update;
    removeRef.current = remove;
  }, [save, update, remove]);

  const subscribeCreated = useCallback((cb: (t: Transaction) => void) => {
    created.current.add(cb);
    return () => {
      created.current.delete(cb);
    };
  }, []);

  const subscribeUpdated = useCallback((cb: (t: Transaction) => void) => {
    updated.current.add(cb);
    return () => {
      updated.current.delete(cb);
    };
  }, []);

  const subscribeDeleted = useCallback((cb: (id: string) => void) => {
    deleted.current.add(cb);
    return () => {
      deleted.current.delete(cb);
    };
  }, []);

  const subscribeRestored = useCallback((cb: (t: Transaction) => void) => {
    restored.current.add(cb);
    return () => {
      restored.current.delete(cb);
    };
  }, []);

  const value = useMemo<SavesValue>(
    () => ({
      pending,
      updating,
      save,
      update,
      remove,
      subscribeCreated,
      subscribeUpdated,
      subscribeDeleted,
      subscribeRestored,
    }),
    [
      pending,
      updating,
      save,
      update,
      remove,
      subscribeCreated,
      subscribeUpdated,
      subscribeDeleted,
      subscribeRestored,
    ],
  );

  return <SavesContext.Provider value={value}>{children}</SavesContext.Provider>;
}

export function useSaves(): SavesValue {
  const ctx = useContext(SavesContext);
  if (!ctx) throw new Error("useSaves must be used inside <SavesProvider>");
  return ctx;
}
