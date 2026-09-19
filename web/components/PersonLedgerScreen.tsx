"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ApiError,
  deleteDebtEntry,
  deletePerson,
  listDebtEntries,
  listPeople,
} from "@/lib/api";
import {
  balanceMeta,
  directionLabel,
  splitDebtEntries,
} from "@/lib/ledger";
import { formatDayHeader, formatTime, karachiDayKey } from "@/lib/date";
import { formatPaisa } from "@/lib/money";
import { friendlyMessage } from "@/lib/useMutation";
import type { DebtEntry, Person } from "@/lib/types";
import {
  AddDebtEntrySheet,
  ConfirmDeleteSheet,
  SettleSheet,
} from "./LedgerSheets";
import { Spinner } from "./Spinner";

type LoadFailure = ApiError | null;

export function PersonLedgerScreen({ personID }: { personID: string }) {
  const router = useRouter();
  const [person, setPerson] = useState<Person | null>(null);
  const [entries, setEntries] = useState<DebtEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<LoadFailure>(null);
  const [entrySheetOpen, setEntrySheetOpen] = useState(false);
  const [settleTarget, setSettleTarget] = useState<DebtEntry[] | null>(null);
  const [settleMode, setSettleMode] = useState<"single" | "all">("single");
  const [entryToDelete, setEntryToDelete] = useState<DebtEntry | null>(null);
  const [deletingPerson, setDeletingPerson] = useState(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setError(null);
      setNotFound(false);
      const [peopleResult, entriesResult] = await Promise.allSettled([
        listPeople(signal),
        listDebtEntries(personID, signal),
      ]);
      if (signal?.aborted) return;

      const entriesError = entriesResult.status === "rejected" ? entriesResult.reason : null;
      if (entriesError instanceof ApiError && entriesError.status === 404) {
        setPerson(null);
        setEntries([]);
        setNotFound(true);
        setLoading(false);
        return;
      }
      if (peopleResult.status === "rejected" || entriesResult.status === "rejected") {
        const reason =
          peopleResult.status === "rejected"
            ? peopleResult.reason
            : entriesResult.status === "rejected"
              ? entriesResult.reason
              : new Error("Ledger requests did not return a result.");
        setError(reason instanceof ApiError ? reason : new ApiError("Failed to load this ledger."));
        setLoading(false);
        return;
      }

      const found = peopleResult.value.find((candidate) => candidate.id === personID);
      if (!found) {
        setPerson(null);
        setEntries([]);
        setNotFound(true);
      } else {
        setPerson(found);
        setEntries(entriesResult.value);
      }
      setLoading(false);
    },
    [personID],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const groups = useMemo(() => splitDebtEntries(entries), [entries]);
  const applyEntriesThenReload = useCallback(
    (changed: DebtEntry[]) => {
      const replacements = new Map(changed.map((entry) => [entry.id, entry]));
      setEntries((current) => current.map((entry) => replacements.get(entry.id) ?? entry));
      void load();
    },
    [load],
  );

  if (notFound) return <MissingPerson />;
  if (loading && person === null) return <LoadingPerson />;
  if (error && person === null) return <PersonError message={friendlyMessage(error)} onRetry={() => void load()} />;
  if (!person) return null;

  const balance = balanceMeta(person.balance_paisa);
  const balanceColour = balance.tone === "owed" ? "text-primary" : balance.tone === "owing" ? "text-brick" : "text-ink-soft";
  const settleEntries = settleTarget ?? [];

  return (
    <div className="px-4 pt-4">
      <Link href="/ledger" className="inline-flex min-h-11 items-center -ml-2 px-2 text-sm font-medium text-primary">
        ← Ledger
      </Link>
      <header className="mt-2 rounded-2xl border border-line bg-paper-raised px-4 py-4">
        <p className="text-label uppercase tracking-widest text-ink-faint">Person</p>
        <div className="mt-1 flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h1 className="truncate text-2xl font-semibold tracking-tight">{person.name}</h1>
            {person.phone ? <a href={`tel:${person.phone}`} className="mt-1 block text-sm text-primary">{person.phone}</a> : null}
          </div>
          <div className="shrink-0 text-right">
            <p className={`tabular text-money-lg font-semibold ${balanceColour}`}>{formatPaisa(person.balance_paisa, { signed: true })}</p>
            <p className="mt-0.5 text-xs text-ink-faint">{balance.label}</p>
          </div>
        </div>
        {person.notes ? <p className="mt-3 border-t border-line pt-3 text-sm leading-relaxed text-ink-soft">{person.notes}</p> : null}
      </header>

      {error ? <div role="alert" className="mt-4 flex items-center justify-between gap-3 rounded-xl bg-brick-tint px-3 py-2.5"><p className="text-sm text-brick">{friendlyMessage(error)} Showing saved details.</p><button type="button" onClick={() => void load()} className="min-h-11 px-2 text-sm font-semibold text-ink">Retry</button></div> : null}

      <div className="mt-4 grid grid-cols-2 gap-2">
        <button type="button" onClick={() => setEntrySheetOpen(true)} className="min-h-11 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink">
          Add entry
        </button>
        <button
          type="button"
          disabled={groups.unsettled.length === 0}
          onClick={() => {
            setSettleMode("all");
            setSettleTarget(groups.unsettled);
          }}
          className="min-h-11 rounded-xl border border-line-strong px-4 text-sm font-semibold text-ink disabled:opacity-40"
        >
          Settle all
        </button>
      </div>

      {groups.unsettled.length ? <EntrySection title="Outstanding" entries={groups.unsettled} onSettle={(entry) => { setSettleMode("single"); setSettleTarget([entry]); }} onDelete={setEntryToDelete} /> : null}
      {groups.settled.length ? <EntrySection title="Settled" entries={groups.settled} dimmed onSettle={() => undefined} onDelete={setEntryToDelete} /> : null}
      {!entries.length ? <NoEntries onAdd={() => setEntrySheetOpen(true)} /> : null}
      {!entries.length ? <button type="button" onClick={() => setDeletingPerson(true)} className="mt-6 min-h-11 w-full rounded-xl border border-brick/40 px-4 text-sm font-semibold text-brick">Delete person</button> : null}

      <AddDebtEntrySheet
        open={entrySheetOpen}
        personID={personID}
        onRequestClose={() => setEntrySheetOpen(false)}
        onCreated={(entry) => {
          setEntries((current) => [entry, ...current]);
          void load();
        }}
      />
      <SettleSheet
        open={settleTarget !== null}
        entries={settleEntries}
        mode={settleMode}
        personID={personID}
        onRequestClose={() => setSettleTarget(null)}
        onSettled={applyEntriesThenReload}
      />
      <ConfirmDeleteSheet
        open={entryToDelete !== null}
        title="Delete entry?"
        description="This permanently removes the debt entry. Settling is usually safer because it keeps the history."
        confirmLabel="Delete entry"
        onRequestClose={() => setEntryToDelete(null)}
        onConfirm={async () => {
          if (!entryToDelete) return;
          await deleteDebtEntry(entryToDelete.id);
          setEntries((current) => current.filter((entry) => entry.id !== entryToDelete.id));
          void load();
        }}
      />
      <ConfirmDeleteSheet
        open={deletingPerson}
        title="Delete person?"
        description="This only works while the person has no debt history."
        confirmLabel="Delete person"
        onRequestClose={() => setDeletingPerson(false)}
        onConfirm={async () => {
          await deletePerson(personID);
          router.replace("/ledger");
        }}
      />
    </div>
  );
}

function EntrySection({ title, entries, dimmed = false, onSettle, onDelete }: { title: string; entries: DebtEntry[]; dimmed?: boolean; onSettle: (entry: DebtEntry) => void; onDelete: (entry: DebtEntry) => void }) {
  return (
    <section className={dimmed ? "mt-7 opacity-65" : "mt-7"}>
      <h2 className="text-label uppercase tracking-widest text-ink-faint">{title}</h2>
      <ul className="mt-2 overflow-hidden rounded-2xl border border-line bg-paper-raised">
        {entries.map((entry, index) => <EntryRow key={entry.id} entry={entry} border={index > 0} onSettle={onSettle} onDelete={onDelete} />)}
      </ul>
    </section>
  );
}

function EntryRow({ entry, border, onSettle, onDelete }: { entry: DebtEntry; border: boolean; onSettle: (entry: DebtEntry) => void; onDelete: (entry: DebtEntry) => void }) {
  const outward = entry.direction === "i_owe";
  return (
    <li className={`px-4 py-3 ${border ? "border-t border-line" : ""}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0"><p className="truncate text-sm font-medium">{entry.description}</p><p className={`mt-0.5 text-xs ${outward ? "text-brick" : "text-primary"}`}>{directionLabel(entry.direction)}</p></div>
        <p className={`tabular shrink-0 font-semibold ${outward ? "text-brick" : "text-primary"}`}>{formatPaisa(entry.amount_paisa)}</p>
      </div>
      <div className="mt-2 flex items-center justify-between gap-3 text-xs text-ink-faint"><span>{formatDayHeader(karachiDayKey(entry.incurred_at))} · {formatTime(entry.incurred_at)}</span>{entry.settled_at ? <span>Paid {formatDayHeader(karachiDayKey(entry.settled_at))}</span> : null}</div>
      <div className="mt-2 flex gap-3">
        {!entry.settled_at ? <button type="button" onClick={() => onSettle(entry)} className="min-h-11 px-2 text-sm font-semibold text-primary">Mark paid</button> : null}
        <button type="button" onClick={() => onDelete(entry)} className="min-h-11 px-2 text-sm font-medium text-brick">Delete</button>
      </div>
    </li>
  );
}

function LoadingPerson() { return <div role="status" className="flex min-h-[50vh] items-center justify-center gap-2 text-sm text-ink-soft"><Spinner /> Loading person…</div>; }
function PersonError({ message, onRetry }: { message: string; onRetry: () => void }) { return <div role="alert" className="px-4 pt-10 text-center"><p className="font-medium text-brick">Couldn&apos;t load this Ledger</p><p className="mt-1 text-sm text-ink-soft">{message}</p><button type="button" onClick={onRetry} className="mt-4 min-h-11 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink">Retry</button></div>; }
function MissingPerson() { return <div className="px-4 pt-12 text-center"><h1 className="text-xl font-semibold">Person not found</h1><p className="mt-2 text-sm text-ink-soft">They may have been deleted in another session.</p><Link href="/ledger" className="mt-5 inline-flex min-h-11 items-center rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink">Back to Ledger</Link></div>; }
function NoEntries({ onAdd }: { onAdd: () => void }) { return <section className="mt-7 rounded-2xl border border-dashed border-line-strong bg-paper-raised px-6 py-7 text-center"><h2 className="font-medium">No entries yet</h2><p className="mt-1.5 text-sm text-ink-soft">Add the first loan, repayment, or amount you owe.</p><button type="button" onClick={onAdd} className="mt-4 min-h-11 rounded-xl border border-line-strong px-4 text-sm font-semibold text-primary">Add entry</button></section>; }
