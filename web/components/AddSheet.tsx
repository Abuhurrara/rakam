"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter } from "next/navigation";
import { isValidAmountInput, paisaToInputString } from "@/lib/money";
import {
  fromKarachiDateInput,
  karachiDateInputValue,
  toKarachiRFC3339,
} from "@/lib/date";
import { orderByMru, pushMru, readMru } from "@/lib/mru";
import { countTap, mark } from "@/lib/perf";
import { useTap } from "@/lib/useTap";
import type { Transaction, TransactionInput } from "@/lib/types";
import { useCategories } from "./CategoriesProvider";
import { useSaves } from "./SavesProvider";
import { Keypad } from "./Keypad";

/**
 * Adding an expense must take under five seconds, one-handed, on a phone.
 * Everything here follows from that:
 *
 *  - It is a sheet inside the layout, not a route, so opening costs no
 *    navigation and no network. Categories are already in memory.
 *  - It opens on the amount with a large keypad, no OS keyboard.
 *  - Category is a chip you tap. Never a dropdown.
 *  - Description and date are optional and out of the way.
 *  - One button saves and closes.
 */

type Mode = { type: "create" } | { type: "edit"; transaction: Transaction };

type AddSheetValue = {
  open: (mode?: Mode) => void;
  close: () => void;
  isOpen: boolean;
};

const AddSheetContext = createContext<AddSheetValue | null>(null);

export function AddSheetProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<Mode | null>(null);

  const open = useCallback((next: Mode = { type: "create" }) => {
    setMode(next);
  }, []);

  const close = useCallback(() => setMode(null), []);

  const value = useMemo<AddSheetValue>(
    () => ({ open, close, isOpen: mode !== null }),
    [open, close, mode],
  );

  return (
    <AddSheetContext.Provider value={value}>
      {children}
      {mode ? <Sheet mode={mode} onClose={close} /> : null}
    </AddSheetContext.Provider>
  );
}

export function useAddSheet(): AddSheetValue {
  const ctx = useContext(AddSheetContext);
  if (!ctx) throw new Error("useAddSheet must be used inside <AddSheetProvider>");
  return ctx;
}

function Sheet({ mode, onClose }: { mode: Mode; onClose: () => void }) {
  const editing = mode.type === "edit" ? mode.transaction : null;
  const { expense: expenseCategories } = useCategories();
  const { save, update, remove } = useSaves();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const tap = useTap();
  const router = useRouter();
  const pathname = usePathname();

  const [amount, setAmount] = useState(() =>
    editing ? paisaToInputString(editing.amount_paisa) : "",
  );
  const [categoryId, setCategoryId] = useState<string | null>(
    () => editing?.category_id ?? null,
  );
  const [description, setDescription] = useState(
    () => editing?.description ?? "",
  );
  const [dateValue, setDateValue] = useState(() =>
    karachiDateInputValue(editing ? new Date(editing.occurred_at) : new Date()),
  );
  const [dateOpen, setDateOpen] = useState(false);

  const today = karachiDateInputValue(new Date());
  const dateChanged = dateValue !== today;
  const canSave = isValidAmountInput(amount);

  // Most-used first. Read once on open so the row never reshuffles under the
  // user's thumb mid-tap.
  const categories = useMemo(
    () => orderByMru(expenseCategories, readMru()),
    [expenseCategories],
  );

  // Marked in a layout effect, which runs synchronously once React has put
  // the sheet in the DOM — at which point the keypad and chips are clickable.
  // Paint follows in the next frame. Deliberately not requestAnimationFrame:
  // a browser throttles rAF when the window is not actively painting, which
  // makes the number meaningless under automation.
  useLayoutEffect(() => {
    mark("sheetReady");
  }, []);

  /*
   * The browser back button closes the sheet rather than leaving the app.
   * Matters most when installed as a PWA, where back is the only gesture.
   *
   * This effect must be safe to run twice. React Strict Mode deliberately
   * runs effects run -> clean up -> run again in development, and an earlier
   * version of this called history.back() in the cleanup: the sheet pushed an
   * entry, the cleanup immediately went back, the effect re-ran and attached a
   * new listener, and that listener caught the pop and closed the sheet. It
   * opened and vanished in a blink.
   *
   * So: the ref guards the push (refs survive Strict Mode's remount), and the
   * cleanup only ever removes its listener. Nothing here navigates.
   */
  const pushedHistoryRef = useRef(false);

  useEffect(() => {
    if (!pushedHistoryRef.current) {
      window.history.pushState({ rakamSheet: true }, "");
      pushedHistoryRef.current = true;
    }
    const onPop = () => {
      pushedHistoryRef.current = false;
      onClose();
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [onClose]);

  /**
   * The single way this sheet closes. Going back through history keeps the
   * entry we pushed balanced, and the popstate listener above does the actual
   * closing — so every route out of the sheet behaves identically.
   */
  const closeSheet = useCallback(() => {
    if (pushedHistoryRef.current) {
      pushedHistoryRef.current = false;
      window.history.back();
    } else {
      onClose();
    }
  }, [onClose]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeSheet();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [closeSheet]);

  function handleSave() {
    if (!canSave) return;
    countTap();
    mark("saveTap");

    const trimmed = description.trim();
    const input: TransactionInput = {
      kind: "expense",
      // The raw string the user typed. Never parsed here — the API does it.
      amount,
      category_id: categoryId,
      description: trimmed === "" ? null : trimmed,
      occurred_at: dateChanged
        ? fromKarachiDateInput(dateValue, new Date())
        : toKarachiRFC3339(new Date()),
    };

    if (categoryId) pushMru(categoryId);

    // Close first. The save continues in the background and reports itself
    // through the list row and, if it fails, a persistent toast.
    closeSheet();

    if (editing) {
      update(editing.id, input);
    } else {
      save(input);
      // Land the user where the new row appears, so the success signal is
      // always visible without needing a success toast. Queued so the history
      // pop from closeSheet() settles before the router navigates.
      if (pathname !== "/expenses") {
        setTimeout(() => router.push("/expenses"), 0);
      }
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex flex-col justify-end">
      <button
        type="button"
        aria-label="Close"
        onClick={closeSheet}
        className="absolute inset-0 bg-overlay"
      />

      {/* overflow-y-auto is a safety net for short viewports: on a tall phone
          nothing scrolls, but a 640px-high screen should scroll rather than
          clip the Save button off the bottom. */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label={editing ? "Edit expense" : "New expense"}
        className="relative flex max-h-[96vh] flex-col overflow-y-auto rounded-t-3xl border-t border-line bg-paper px-4 pt-3 pb-[calc(env(safe-area-inset-bottom,0px)+0.75rem)] shadow-2xl"
      >
        <div className="mb-1 flex items-center justify-between">
          <button
            type="button"
            onClick={closeSheet}
            className="-ml-2 min-h-11 px-2 text-sm text-ink-soft"
          >
            Cancel
          </button>
          <span className="text-label uppercase tracking-widest text-ink-faint">
            {editing ? "Edit expense" : "New expense"}
          </span>
          {editing ? (
            <button
              type="button"
              onClick={() => {
                if (!confirmingDelete) {
                  setConfirmingDelete(true);
                  return;
                }
                closeSheet();
                remove(editing);
              }}
              className={`-mr-2 min-h-11 px-2 text-sm ${
                confirmingDelete ? "font-semibold text-brick" : "text-ink-soft"
              }`}
            >
              {confirmingDelete ? "Sure?" : "Delete"}
            </button>
          ) : (
            <span className="min-h-11 w-12" aria-hidden="true" />
          )}
        </div>

        <AmountDisplay amount={amount} />

        <CategoryChips
          categories={categories}
          selectedId={categoryId}
          onSelect={(id) => {
            countTap();
            setCategoryId((current) => (current === id ? null : id));
          }}
        />

        <div className="mb-2.5 mt-2.5 flex items-center gap-2">
          <input
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Note (optional)"
            enterKeyHint="done"
            className="min-h-11 min-w-0 flex-1 rounded-xl border border-line bg-paper-raised px-3 text-sm text-ink placeholder:text-ink-faint focus:border-primary focus:outline-none"
          />
          {dateOpen ? (
            <input
              type="date"
              value={dateValue}
              max={today}
              onChange={(e) => setDateValue(e.target.value)}
              className="tabular min-h-11 rounded-xl border border-line bg-paper-raised px-2 text-sm text-ink focus:border-primary focus:outline-none"
            />
          ) : (
            <button
              type="button"
              onClick={() => setDateOpen(true)}
              className="min-h-11 shrink-0 rounded-xl border border-line px-3 text-sm text-ink-soft"
            >
              {dateChanged ? dateValue : "Today"}
              <span className="ml-1.5 text-ink-faint underline">change</span>
            </button>
          )}
        </div>

        <Keypad onKey={(next) => setAmount(next)} />

        <button
          type="button"
          disabled={!canSave}
          {...tap(handleSave)}
          className="mt-2.5 min-h-[3.25rem] w-full rounded-2xl bg-primary text-base font-semibold text-primary-ink transition-opacity disabled:opacity-35"
        >
          {editing ? "Save changes" : "Save expense"}
        </button>
      </div>
    </div>
  );
}

function AmountDisplay({ amount }: { amount: string }) {
  const empty = amount === "";
  return (
    <div className="flex items-baseline justify-center gap-1.5 py-3">
      <span className="text-lg font-medium text-ink-faint">Rs</span>
      <span
        className={`tabular text-money-hero font-semibold ${
          empty ? "text-ink-faint" : "text-ink"
        }`}
        aria-live="polite"
        aria-label={`Amount ${empty ? "empty" : amount} rupees`}
      >
        {empty ? "0" : amount}
      </span>
      <span
        className="ml-0.5 inline-block h-9 w-0.5 animate-pulse bg-primary"
        aria-hidden="true"
      />
    </div>
  );
}

function CategoryChips({
  categories,
  selectedId,
  onSelect,
}: {
  categories: { id: string; name: string; icon: string; color: string }[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}) {
  const tap = useTap();

  if (categories.length === 0) {
    return (
      <p className="py-2 text-center text-sm text-ink-faint">
        No categories yet.
      </p>
    );
  }

  return (
    <div
      className="no-scrollbar -mx-1 flex max-h-[5.5rem] flex-wrap gap-1.5 overflow-y-auto px-1"
      role="group"
      aria-label="Category"
    >
      {categories.map((c) => {
        const selected = c.id === selectedId;
        return (
          <button
            key={c.id}
            type="button"
            aria-pressed={selected}
            {...tap(() => onSelect(c.id))}
            className={`flex min-h-11 items-center gap-1.5 rounded-full border px-3 text-sm transition-colors ${
              selected
                ? "border-primary bg-primary-tint font-medium text-ink"
                : "border-line bg-paper-raised text-ink-soft"
            }`}
          >
            <span aria-hidden="true">{c.icon}</span>
            {c.name}
          </button>
        );
      })}
    </div>
  );
}
