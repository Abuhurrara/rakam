"use client";

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import {
  createDebtEntry,
  createPerson,
  settleAllDebtEntries,
  settleDebtEntry,
} from "@/lib/api";
import { fromKarachiDateInput, karachiDateInputValue } from "@/lib/date";
import {
  categoryKindForDirection,
  commonDirection,
  directionLabel,
} from "@/lib/ledger";
import { isValidAmountInput } from "@/lib/money";
import { friendlyMessage, useMutation } from "@/lib/useMutation";
import type {
  DebtDirection,
  DebtEntry,
  DebtEntryInput,
  Person,
  PersonInput,
  SettleAllResult,
  Settlement,
  SettlementInput,
} from "@/lib/types";
import { useCategories } from "./CategoriesProvider";

type SheetProps = {
  open: boolean;
  onRequestClose: () => void;
  title: string;
  children: ReactNode;
};

/** A small, dependency-free mobile sheet. Native dialog supplies focus safety. */
function LedgerSheet({ open, onRequestClose, title, children }: SheetProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleID = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={titleID}
      onCancel={(event) => {
        event.preventDefault();
        onRequestClose();
      }}
      onClose={() => {
        if (open) onRequestClose();
      }}
      className="fixed inset-x-0 bottom-0 m-0 w-full max-w-lg rounded-t-3xl border border-line bg-paper p-0 text-ink shadow-2xl backdrop:bg-overlay"
    >
      <div className="max-h-[92dvh] overflow-y-auto px-4 pt-4 pb-[calc(env(safe-area-inset-bottom,0px)+1rem)]">
        <div className="flex items-center justify-between gap-3">
          <h2 id={titleID} className="text-lg font-semibold">
            {title}
          </h2>
          <button
            type="button"
            onClick={onRequestClose}
            aria-label="Close"
            className="-mr-2 min-h-11 min-w-11 rounded-full text-xl text-ink-soft"
          >
            ×
          </button>
        </div>
        {children}
      </div>
    </dialog>
  );
}

function FormError({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <p role="alert" className="mt-3 rounded-xl bg-brick-tint px-3 py-2 text-sm text-brick">
      {message}
    </p>
  );
}

export function AddPersonSheet({
  open,
  onRequestClose,
  onCreated,
}: {
  open: boolean;
  onRequestClose: () => void;
  onCreated: (person: Person) => void;
}) {
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [notes, setNotes] = useState("");
  const { status, error, run, reset } = useMutation<PersonInput, Person>(createPerson);

  useEffect(() => {
    if (!open) return;
    setName("");
    setPhone("");
    setNotes("");
    reset();
  }, [open, reset]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName || status === "pending") return;
    const result = await run({
      name: trimmedName,
      phone: phone.trim() || null,
      notes: notes.trim() || null,
    });
    if (result.ok) {
      onCreated(result.data);
      onRequestClose();
    }
  }

  return (
    <LedgerSheet open={open} onRequestClose={onRequestClose} title="Add person">
      <form onSubmit={submit} className="mt-4 space-y-3">
        <label className="block text-sm font-medium">
          Name
          <input
            autoFocus
            required
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="e.g. Usman"
            className="mt-1.5 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3 text-ink"
          />
        </label>
        <label className="block text-sm font-medium">
          Phone <span className="font-normal text-ink-faint">optional</span>
          <input
            value={phone}
            onChange={(event) => setPhone(event.target.value)}
            inputMode="tel"
            placeholder="03xx xxx xxxx"
            className="mt-1.5 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3 text-ink"
          />
        </label>
        <label className="block text-sm font-medium">
          Notes <span className="font-normal text-ink-faint">optional</span>
          <textarea
            value={notes}
            onChange={(event) => setNotes(event.target.value)}
            rows={3}
            className="mt-1.5 w-full rounded-xl border border-line bg-paper-raised px-3 py-2 text-ink"
          />
        </label>
        <FormError message={error ? friendlyMessage(error) : null} />
        <button
          type="submit"
          disabled={!name.trim() || status === "pending"}
          className="min-h-11 w-full rounded-xl bg-primary px-4 font-semibold text-primary-ink disabled:opacity-40"
        >
          {status === "pending" ? "Saving…" : "Add person"}
        </button>
      </form>
    </LedgerSheet>
  );
}

export function AddDebtEntrySheet({
  open,
  personID,
  onRequestClose,
  onCreated,
}: {
  open: boolean;
  personID: string;
  onRequestClose: () => void;
  onCreated: (entry: DebtEntry) => void;
}) {
  const [direction, setDirection] = useState<DebtDirection>("they_owe");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");
  const [dateValue, setDateValue] = useState(() => karachiDateInputValue(new Date()));
  const saveEntry = useCallback(
    (input: DebtEntryInput) => createDebtEntry(personID, input),
    [personID],
  );
  const { status, error, run, reset } = useMutation<DebtEntryInput, DebtEntry>(saveEntry);

  useEffect(() => {
    if (!open) return;
    setDirection("they_owe");
    setAmount("");
    setDescription("");
    setDateValue(karachiDateInputValue(new Date()));
    reset();
  }, [open, reset]);

  const today = karachiDateInputValue(new Date());
  const canSave =
    isValidAmountInput(amount) &&
    description.trim() !== "" &&
    /^\d{4}-\d{2}-\d{2}$/.test(dateValue) &&
    dateValue <= today;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSave || status === "pending") return;
    const result = await run({
      direction,
      amount: amount.trim(),
      description: description.trim(),
      incurred_at: fromKarachiDateInput(dateValue, new Date()),
    });
    if (result.ok) {
      onCreated(result.data);
      onRequestClose();
    }
  }

  return (
    <LedgerSheet open={open} onRequestClose={onRequestClose} title="Add debt entry">
      <form onSubmit={submit} className="mt-4 space-y-4">
        <fieldset>
          <legend className="text-sm font-medium">Who owes whom?</legend>
          <div className="mt-2 grid grid-cols-2 gap-2">
            <button
              type="button"
              onClick={() => setDirection("they_owe")}
              aria-pressed={direction === "they_owe"}
              className={`min-h-11 rounded-xl border px-3 text-sm font-medium ${
                direction === "they_owe"
                  ? "border-primary bg-primary-tint text-primary"
                  : "border-line bg-paper-raised"
              }`}
            >
              They owe you
            </button>
            <button
              type="button"
              onClick={() => setDirection("i_owe")}
              aria-pressed={direction === "i_owe"}
              className={`min-h-11 rounded-xl border px-3 text-sm font-medium ${
                direction === "i_owe"
                  ? "border-brick bg-brick-tint text-brick"
                  : "border-line bg-paper-raised"
              }`}
            >
              You owe them
            </button>
          </div>
        </fieldset>
        <label className="block text-sm font-medium">
          Amount
          <input
            autoFocus
            required
            inputMode="decimal"
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
            placeholder="0.00"
            className="tabular mt-1.5 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3 text-ink"
          />
        </label>
        <label className="block text-sm font-medium">
          What was it for?
          <input
            required
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="e.g. Dinner"
            className="mt-1.5 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3 text-ink"
          />
        </label>
        <label className="block text-sm font-medium">
          Date
          <input
            required
            type="date"
            max={today}
            value={dateValue}
            onChange={(event) => setDateValue(event.target.value)}
            className="tabular mt-1.5 min-h-11 w-full rounded-xl border border-line bg-paper-raised px-3 text-ink"
          />
        </label>
        <FormError message={error ? friendlyMessage(error) : null} />
        <button
          type="submit"
          disabled={!canSave || status === "pending"}
          className="min-h-11 w-full rounded-xl bg-primary px-4 font-semibold text-primary-ink disabled:opacity-40"
        >
          {status === "pending" ? "Saving…" : "Add entry"}
        </button>
      </form>
    </LedgerSheet>
  );
}

export function SettleSheet({
  open,
  entries,
  mode,
  personID,
  onRequestClose,
  onSettled,
}: {
  open: boolean;
  entries: DebtEntry[];
  mode: "single" | "all";
  personID: string;
  onRequestClose: () => void;
  onSettled: (entries: DebtEntry[]) => void;
}) {
  const [recordMovement, setRecordMovement] = useState(false);
  const [categoryID, setCategoryID] = useState<string | null>(null);
  const { all: categories, loading: categoriesLoading } = useCategories();
  const direction = useMemo(() => commonDirection(entries), [entries]);
  const compatibleCategories = useMemo(
    () =>
      direction
        ? categories.filter((category) => category.kind === categoryKindForDirection(direction))
        : [],
    [categories, direction],
  );
  const settle = useCallback(
    (input: SettlementInput) =>
      mode === "single"
        ? settleDebtEntry(entries[0]!.id, input)
        : settleAllDebtEntries(personID, input),
    [entries, mode, personID],
  );
  const { status, error, run, reset } = useMutation<SettlementInput, Settlement | SettleAllResult>(settle);

  useEffect(() => {
    if (!open) return;
    setRecordMovement(false);
    setCategoryID(null);
    reset();
  }, [open, reset]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (status === "pending") return;
    const result = await run({
      create_transaction: recordMovement,
      ...(recordMovement && direction ? { category_id: categoryID } : {}),
    });
    if (!result.ok) return;
    const settled =
      mode === "single"
        ? (result.data as Settlement).debt_entry
        : (result.data as SettleAllResult).debt_entries;
    onSettled(Array.isArray(settled) ? settled : [settled]);
    onRequestClose();
  }

  const count = entries.length;
  const title = mode === "single" ? "Mark as paid" : "Settle all";

  return (
    <LedgerSheet open={open} onRequestClose={onRequestClose} title={title}>
      <form onSubmit={submit} className="mt-4 space-y-4">
        <p className="text-sm leading-relaxed text-ink-soft">
          {mode === "single"
            ? "This marks the debt as paid."
            : `This marks ${count} debt ${count === 1 ? "entry" : "entries"} as paid separately; it never combines balances into one transaction.`}
        </p>
        <label className="flex min-h-11 items-center gap-3 rounded-xl border border-line bg-paper-raised px-3 text-sm font-medium">
          <input
            type="checkbox"
            checked={recordMovement}
            onChange={(event) => setRecordMovement(event.target.checked)}
            className="h-4 w-4 accent-primary"
          />
          Also record the money movement
        </label>
        {recordMovement && direction ? (
          <fieldset>
            <legend className="text-sm font-medium">
              Category <span className="font-normal text-ink-faint">optional</span>
            </legend>
            <p className="mt-1 text-xs text-ink-soft">
              This creates {direction === "they_owe" ? "income" : "an expense"} for the payment.
            </p>
            <div className="no-scrollbar mt-2 flex gap-2 overflow-x-auto pb-1">
              <CategoryChip
                label="No category"
                active={categoryID === null}
                onClick={() => setCategoryID(null)}
              />
              {compatibleCategories.map((category) => (
                <CategoryChip
                  key={category.id}
                  label={`${category.icon} ${category.name}`}
                  active={categoryID === category.id}
                  onClick={() => setCategoryID(category.id)}
                />
              ))}
            </div>
            {categoriesLoading ? <p className="mt-2 text-xs text-ink-faint">Loading categories…</p> : null}
          </fieldset>
        ) : null}
        {recordMovement && !direction ? (
          <p className="rounded-xl bg-gold-tint px-3 py-2 text-sm text-ink-soft">
            These entries have both directions. Each will create its own income or expense transaction without a category.
          </p>
        ) : null}
        <FormError message={error ? friendlyMessage(error) : null} />
        <button
          type="submit"
          disabled={status === "pending"}
          className="min-h-11 w-full rounded-xl bg-primary px-4 font-semibold text-primary-ink disabled:opacity-40"
        >
          {status === "pending" ? "Saving…" : title}
        </button>
      </form>
    </LedgerSheet>
  );
}

function CategoryChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`min-h-11 shrink-0 rounded-full border px-3 text-sm ${
        active
          ? "border-primary bg-primary-tint font-medium text-primary"
          : "border-line bg-paper-raised text-ink-soft"
      }`}
    >
      {label}
    </button>
  );
}

export function ConfirmDeleteSheet({
  open,
  title,
  description,
  confirmLabel,
  onRequestClose,
  onConfirm,
}: {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  onRequestClose: () => void;
  onConfirm: () => Promise<void>;
}) {
  const wasOpen = useRef(false);
  const confirm = useCallback(async () => {
    await onConfirm();
    return null;
  }, [onConfirm]);
  const { status, error, run, reset } = useMutation<void, null>(confirm);

  useEffect(() => {
    if (open && !wasOpen.current) reset();
    wasOpen.current = open;
  }, [open, reset]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (status === "pending") return;
    const result = await run(undefined);
    if (result.ok) onRequestClose();
  }

  return (
    <LedgerSheet open={open} onRequestClose={onRequestClose} title={title}>
      <form onSubmit={submit} className="mt-4 space-y-4">
        <p className="text-sm leading-relaxed text-ink-soft">{description}</p>
        <FormError message={error ? friendlyMessage(error) : null} />
        <div className="grid grid-cols-2 gap-2">
          <button
            type="button"
            onClick={onRequestClose}
            className="min-h-11 rounded-xl border border-line-strong px-4 font-medium"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={status === "pending"}
            className="min-h-11 rounded-xl bg-brick px-4 font-semibold text-white disabled:opacity-40"
          >
            {status === "pending" ? "Deleting…" : confirmLabel}
          </button>
        </div>
      </form>
    </LedgerSheet>
  );
}

export { directionLabel };
