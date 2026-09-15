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
import {
  readDrafts,
  storeDraft,
  removeDraft,
  type SaveDraft,
} from "@/lib/save-drafts";
import type { Transaction, TransactionInput } from "@/lib/types";
import { useFinanceData } from "./FinanceDataProvider";
import { useToast } from "./Toast";

export type PendingSave = { key: string; input: TransactionInput };
type DraftState = SaveDraft & { saving: boolean; error?: string };
type SavesValue = {
  pending: PendingSave[];
  updating: ReadonlySet<string>;
  revision: number;
  save: (input: TransactionInput) => boolean;
  update: (id: string, input: TransactionInput) => boolean;
  remove: (t: Transaction) => void;
};
const SavesContext = createContext<SavesValue | null>(null);

export function SavesProvider({
  userID,
  children,
}: {
  userID: string | null;
  children: ReactNode;
}) {
  const [drafts, setDrafts] = useState<DraftState[]>([]);
  const [revision, setRevision] = useState(0);
  const [storageError, setStorageError] = useState<string | null>(null);
  const [discarding, setDiscarding] = useState<string | null>(null);
  const inFlight = useRef(new Set<string>());
  const toast = useToast();
  const financeData = useFinanceData();

  useEffect(() => {
    if (!userID) return;
    try {
      setDrafts(
        readDrafts(localStorage, userID).map((d) => ({ ...d, saving: false })),
      );
    } catch {
      setStorageError(
        "Saved drafts could not be read. Keep this app open and check your browser storage settings.",
      );
    }
  }, [userID]);

  const run = useCallback(
    async (draft: SaveDraft) => {
      if (!userID || inFlight.current.has(draft.key)) return;
      inFlight.current.add(draft.key);
      setDrafts((ds) =>
        ds.map((d) =>
          d.key === draft.key ? { ...d, saving: true, error: undefined } : d,
        ),
      );
      try {
        const saved = draft.transactionID
          ? await updateTransaction(draft.transactionID, draft.input)
          : await createTransaction(draft.input, draft.key);
        financeData.recordSaved(saved, draft.transactionID);
        // Clear only after a definite server acknowledgement. If storage removal
        // fails, the stable request key still makes a later create retry safe.
        removeDraft(localStorage, userID, draft.key);
        setDrafts((ds) => ds.filter((d) => d.key !== draft.key));
        mark("saveDone");
        reportAddFlow("saved");
      } catch (err) {
        const e =
          err instanceof ApiError
            ? err
            : new ApiError("Could not finish saving. Your draft is retained.");
        setDrafts((ds) =>
          ds.map((d) =>
            d.key === draft.key
              ? { ...d, saving: false, error: friendlyMessage(e) }
              : d,
          ),
        );
        mark("saveDone");
        reportAddFlow("failed");
      } finally {
        inFlight.current.delete(draft.key);
        financeData.invalidate();
        // Reload the current filtered page and server total even after an
        // uncertain response: the write may have committed before disconnecting.
        setRevision((n) => n + 1);
      }
    },
    [userID, financeData],
  );

  const enqueue = useCallback(
    (input: TransactionInput, transactionID?: string): boolean => {
      if (!userID || storageError) return false;
      try {
        if (
          transactionID &&
          readDrafts(localStorage, userID).some(
            (d) => d.transactionID === transactionID,
          )
        ) {
          toast.show({
            kind: "error",
            message: "There is already an unfinished change for this expense.",
            detail: "Retry or discard that draft first.",
          });
          return false;
        }
        const draft: SaveDraft = {
          version: 1,
          key: crypto.randomUUID(),
          input,
          transactionID,
        };
        storeDraft(localStorage, userID, draft);
        setDrafts((ds) => [...ds, { ...draft, saving: true }]);
        void run(draft);
        return true;
      } catch {
        toast.show({
          kind: "error",
          message: "Could not protect this entry before saving.",
          detail:
            "Your entry is still open. Free some browser storage, then try again.",
        });
        return false;
      }
    },
    [userID, storageError, run, toast],
  );

  const save = useCallback(
    (input: TransactionInput) => enqueue(input),
    [enqueue],
  );
  const update = useCallback(
    (id: string, input: TransactionInput) => enqueue(input, id),
    [enqueue],
  );
  const remove = useCallback(
    (t: Transaction) => {
      void deleteTransaction(t.id)
        .then(() => financeData.recordDeleted(t))
        .catch((err: unknown) => {
          if (
            err instanceof ApiError &&
            (err.status === 404 || err.status === 401)
          )
            return;
          toast.show({
            kind: "error",
            message: "Couldn't delete that expense. Try again from the list.",
          });
        })
        .finally(() => {
          financeData.invalidate();
          setRevision((n) => n + 1);
        });
    },
    [toast, financeData],
  );

  const value = useMemo<SavesValue>(
    () => ({
      pending: drafts.filter((d) => d.saving && !d.transactionID),
      updating: new Set(
        drafts
          .filter((d) => d.saving && d.transactionID)
          .map((d) => d.transactionID!),
      ),
      revision,
      save,
      update,
      remove,
    }),
    [drafts, revision, save, update, remove],
  );

  const unfinished = drafts.filter((d) => !d.saving);
  return (
    <SavesContext.Provider value={value}>
      {storageError ? (
        <p role="alert" className="mx-auto max-w-lg p-4 text-brick">
          {storageError}
        </p>
      ) : null}
      {unfinished.length ? (
        <section
          aria-label="Unfinished saves"
          className="mx-auto max-w-lg space-y-2 px-4 pt-4"
        >
          <p className="text-sm font-medium">
            These entries need your attention
          </p>
          {unfinished.map((d) => (
            <div
              key={d.key}
              className="rounded-xl border border-line bg-paper-raised p-3"
            >
              <p className="text-sm">
                {d.transactionID ? "Change to" : "Entry for"} Rs{" "}
                {d.input.amount}
                {d.input.description ? ` · ${d.input.description}` : ""}
              </p>
              <p className="mt-1 text-xs text-ink-soft">
                {d.error ??
                  "The app closed before this save was confirmed. Retry safely to check or finish it."}
              </p>
              <div className="mt-2 flex gap-3">
                <button
                  type="button"
                  className="min-h-11 rounded-lg bg-primary px-4 text-primary-ink"
                  onClick={() => void run(d)}
                >
                  Retry
                </button>
                <button
                  type="button"
                  className="min-h-11 px-3 text-brick"
                  onClick={() => {
                    if (discarding !== d.key) {
                      setDiscarding(d.key);
                      return;
                    }
                    try {
                      removeDraft(localStorage, userID!, d.key);
                      setDrafts((ds) => ds.filter((x) => x.key !== d.key));
                      setDiscarding(null);
                    } catch {
                      setStorageError(
                        "Could not discard that draft. It has been kept.",
                      );
                    }
                  }}
                >
                  {discarding === d.key ? "Confirm discard" : "Discard draft"}
                </button>
              </div>
              {discarding === d.key ? (
                <p className="text-xs text-ink-soft">
                  This removes only the local draft. An entry already saved on
                  the server stays in your ledger.
                </p>
              ) : null}
            </div>
          ))}
        </section>
      ) : null}
      {children}
    </SavesContext.Provider>
  );
}
export function useSaves(): SavesValue {
  const ctx = useContext(SavesContext);
  if (!ctx) throw new Error("useSaves must be used inside <SavesProvider>");
  return ctx;
}
