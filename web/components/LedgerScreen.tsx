"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError, listPeople } from "@/lib/api";
import { balanceMeta } from "@/lib/ledger";
import { isLedgerFresh } from "@/lib/ledger-cache";
import { formatPaisa } from "@/lib/money";
import { friendlyMessage } from "@/lib/useMutation";
import type { Person } from "@/lib/types";
import { AddPersonSheet } from "./LedgerSheets";
import { useLedgerData } from "./LedgerDataProvider";
import { Spinner } from "./Spinner";

export function LedgerScreen() {
  const ledgerData = useLedgerData();
  const [people, setPeople] = useState<Person[] | null>(
    () => ledgerData.readPeople()?.data ?? null,
  );
  const [error, setError] = useState<ApiError | null>(null);
  const [loading, setLoading] = useState(
    () => ledgerData.readPeople() === null,
  );
  const [reloadNonce, setReloadNonce] = useState(0);
  const [addingPerson, setAddingPerson] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    const cached = ledgerData.readPeople();
    if (cached) setPeople(cached.data);
    if (reloadNonce === 0 && cached && isLedgerFresh(cached)) {
      setLoading(false);
      setError(null);
      return () => controller.abort();
    }
    // Keep stale data visible while the request confirms it in the background.
    setLoading(cached === null);
    setError(null);

    listPeople(controller.signal)
      .then((next) => {
        if (!cancelled) {
          ledgerData.writePeople(next);
          setPeople(next);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err : new ApiError("Failed to load people."));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [reloadNonce, ledgerData]);

  const reload = useCallback(() => setReloadNonce((value) => value + 1), []);

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") reload();
    };
    window.addEventListener("online", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      window.removeEventListener("online", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [reload]);

  return (
    <div className="px-4 pt-4" aria-busy={loading}>
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="text-label uppercase tracking-widest text-ink-faint">People</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">Ledger</h1>
          <p className="mt-1 text-sm text-ink-soft">What you owe and what is owed to you.</p>
        </div>
        <button
          type="button"
          onClick={() => setAddingPerson(true)}
          className="min-h-11 shrink-0 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink"
        >
          Add person
        </button>
      </header>

      {loading && people === null ? <LoadingPeople /> : null}
      {error && people === null ? (
        <ErrorState message={friendlyMessage(error)} onRetry={reload} />
      ) : null}
      {error && people !== null ? (
        <div role="alert" className="mt-4 flex items-center justify-between gap-3 rounded-xl bg-brick-tint px-3 py-2.5">
          <p className="text-sm text-brick">{friendlyMessage(error)} Showing your last list.</p>
          <button type="button" onClick={reload} className="min-h-11 px-2 text-sm font-semibold text-ink">
            Retry
          </button>
        </div>
      ) : null}
      {people !== null && people.length === 0 ? <EmptyState onAdd={() => setAddingPerson(true)} /> : null}
      {people && people.length > 0 ? <PeopleList people={people} /> : null}

      <AddPersonSheet
        open={addingPerson}
        onRequestClose={() => setAddingPerson(false)}
        onCreated={(person) => {
          setPeople((current) => {
            const next = current ? [...current, person] : [person];
            ledgerData.writePeople(next);
            return next;
          });
        }}
      />
    </div>
  );
}

function PeopleList({ people }: { people: Person[] }) {
  return (
    <ul className="mt-5 overflow-hidden rounded-2xl border border-line bg-paper-raised">
      {people.map((person, index) => {
        const balance = balanceMeta(person.balance_paisa);
        const colour =
          balance.tone === "owed"
            ? "text-primary"
            : balance.tone === "owing"
              ? "text-brick"
              : "text-ink-soft";
        return (
          <li key={person.id} className={index ? "border-t border-line" : undefined}>
            <Link
              href={`/ledger/${encodeURIComponent(person.id)}`}
              className="flex min-h-[4.75rem] items-center justify-between gap-4 px-4 py-3 active:bg-paper-sunken"
            >
              <span className="min-w-0">
                <span className="block truncate font-medium text-ink">{person.name}</span>
                <span className="mt-0.5 block text-xs text-ink-faint">{balance.label}</span>
              </span>
              <span className={`tabular shrink-0 text-money font-semibold ${colour}`}>
                {formatPaisa(person.balance_paisa, { signed: true })}
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

function LoadingPeople() {
  return (
    <div role="status" className="mt-8 flex items-center justify-center gap-2 text-sm text-ink-soft">
      <Spinner /> Loading ledger…
    </div>
  );
}

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div role="alert" className="mt-8 rounded-2xl border border-brick/30 bg-brick-tint px-5 py-6 text-center">
      <p className="font-medium text-brick">Couldn&apos;t load Ledger</p>
      <p className="mt-1 text-sm text-ink-soft">{message}</p>
      <button type="button" onClick={onRetry} className="mt-4 min-h-11 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink">
        Retry
      </button>
    </div>
  );
}

function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <section className="mt-8 rounded-2xl border border-dashed border-line-strong bg-paper-raised px-6 py-8 text-center">
      <h2 className="font-medium">No people yet</h2>
      <p className="mt-1.5 text-sm leading-relaxed text-ink-soft">Add someone when you lend, borrow, or need to keep a balance.</p>
      <button type="button" onClick={onAdd} className="mt-5 min-h-11 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink">
        Add your first person
      </button>
    </section>
  );
}
