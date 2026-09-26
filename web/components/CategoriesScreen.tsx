"use client";

import { useState, type FormEvent } from "react";
import {
  ApiError,
  archiveCategory as archiveCategoryRequest,
  createCategory,
  restoreCategory as restoreCategoryRequest,
  updateCategory,
} from "@/lib/api";
import type { Category, CategoryInput, Kind } from "@/lib/types";
import { friendlyMessage } from "@/lib/useMutation";
import { useCategories } from "./CategoriesProvider";
import { Spinner } from "./Spinner";

const ICONS: Record<Kind, string[]> = {
  expense: ["🍔", "🛒", "🚗", "🏠", "💡", "📱", "🛍️", "🩺", "✈️", "🎬", "👨‍👩‍👧", "🔖", "🐾", "📚", "💻"],
  income: ["💰", "💻", "📈", "🎁", "🏦", "🧾", "🔖", "✨"],
};

const COLORS = [
  "#2E7D32", "#8B4513", "#455A64", "#5D4037", "#F9A825",
  "#0277BD", "#AD1457", "#C62828", "#00838F", "#6A1B9A", "#EF6C00", "#616161",
];

function errorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 409) {
    if (error.message.toLowerCase().includes("active recurring bill")) {
      return "Change or deactivate the recurring bill using this category, then try again.";
    }
    return "A category with that name already exists for this type.";
  }
  return error instanceof ApiError ? friendlyMessage(error) : "Something went wrong. Please try again.";
}

export function CategoriesScreen() {
  const { all, loading, error: loadError, reload, saveLocal, archiveLocal } = useCategories();
  const [kind, setKind] = useState<Kind>("expense");
  const [editingID, setEditingID] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [icon, setIcon] = useState(ICONS.expense[0]!);
  const [color, setColor] = useState(COLORS[0]!);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [archiveArmed, setArchiveArmed] = useState<string | null>(null);

  const editing = all.find((category) => category.id === editingID) ?? null;
  const visible = all.filter((category) => category.kind === kind && !category.is_archived);
  const archived = all.filter((category) => category.kind === kind && category.is_archived);
  const colorOptions = COLORS.includes(color) ? COLORS : [color, ...COLORS];

  function resetForm() {
    setEditingID(null);
    setAdding(false);
    setName("");
    setIcon(ICONS[kind][0]!);
    setColor(COLORS[0]!);
    setError("");
  }

  function startCreate() {
    setEditingID(null);
    setAdding(true);
    setName("");
    setIcon(ICONS[kind][0]!);
    setColor(COLORS[0]!);
    setError("");
  }

  function startEdit(category: Category) {
    setAdding(false);
    setEditingID(category.id);
    setName(category.name);
    setIcon(category.icon);
    setColor(category.color);
    setError("");
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const normalizedName = name.trim();
    if (!normalizedName) {
      setError("Enter a category name.");
      return;
    }

    const sortOrder = editing?.sort_order ??
      Math.max(-1, ...all.filter((category) => category.kind === kind).map((category) => category.sort_order)) + 1;
    const input: CategoryInput = { name: normalizedName, kind, icon, color, sort_order: sortOrder };
    setBusy(true);
    setError("");
    try {
      const saved = editing ? await updateCategory(editing.id, input) : await createCategory(input);
      saveLocal(saved);
      resetForm();
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  }

  async function archive(category: Category) {
    if (archiveArmed !== category.id) {
      setArchiveArmed(category.id);
      setError("");
      return;
    }
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await archiveCategoryRequest(category.id);
      archiveLocal(category.id);
      setArchiveArmed(null);
      if (editingID === category.id) resetForm();
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  }

  async function restore(category: Category) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      saveLocal(await restoreCategoryRequest(category.id));
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="px-4 pb-8 pt-5">
      <header>
        <p className="text-label uppercase tracking-widest text-ink-faint">Make it yours</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight text-ink">Categories</h1>
        <p className="mt-1 text-sm leading-relaxed text-ink-soft">
          Archived categories stay on old entries, but won&apos;t appear when you add new ones.
        </p>
      </header>

      <div className="mt-5 grid grid-cols-2 rounded-xl border border-line bg-paper-sunken p-1" role="group" aria-label="Category type">
        {(["expense", "income"] as const).map((item) => (
          <button
            key={item}
            type="button"
            aria-pressed={kind === item}
            onClick={() => {
              setKind(item);
              resetForm();
              setArchiveArmed(null);
            }}
            className={`min-h-11 rounded-lg text-sm font-medium capitalize focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary ${kind === item ? "bg-paper-raised text-ink shadow-sm" : "text-ink-soft"}`}
          >
            {item}
          </button>
        ))}
      </div>

      {loadError ? (
        <div role="alert" className="mt-4 flex items-center justify-between gap-3 rounded-xl bg-brick-tint px-3 py-2.5">
          <p className="text-sm text-brick">Couldn&apos;t load categories: {friendlyMessage(loadError)}</p>
          <button type="button" onClick={reload} className="min-h-11 shrink-0 px-2 text-sm font-semibold text-ink">Retry</button>
        </div>
      ) : null}
      {error ? <p role="alert" className="mt-4 rounded-xl bg-brick-tint px-3 py-2.5 text-sm text-brick">{error}</p> : null}
      {loading && all.length === 0 ? <div role="status" className="mt-8 flex justify-center gap-2 text-sm text-ink-soft"><Spinner /> Loading categories…</div> : null}

      <section className="mt-5" aria-label={`${kind} categories`}>
        <div className="flex items-center justify-between">
          <h2 className="text-label font-semibold uppercase tracking-widest text-ink-faint">Active · {visible.length}</h2>
          {!adding && !editing ? (
            <button type="button" onClick={startCreate} className="min-h-11 rounded-full bg-primary px-4 text-sm font-semibold text-primary-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary">
              Add category
            </button>
          ) : null}
        </div>

        {adding || editing ? (
          <form onSubmit={save} className="mt-3 rounded-2xl border border-line bg-paper-raised p-4">
            <h3 className="font-semibold text-ink">{editing ? "Edit category" : "New category"}</h3>
            <label className="mt-3 block text-sm font-medium text-ink">
              Name
              <input
                autoFocus
                maxLength={40}
                value={name}
                onChange={(event) => setName(event.target.value)}
                className="mt-1 min-h-12 w-full rounded-xl border border-line-strong bg-paper px-3 text-base text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
                placeholder="e.g. Coffee"
                required
              />
            </label>
            <fieldset className="mt-4">
              <legend className="text-sm font-medium text-ink">Choose an emoji</legend>
              <div className="mt-2 flex flex-wrap gap-2">
                {ICONS[kind].map((choice) => (
                  <button
                    key={choice}
                    type="button"
                    aria-label={`Use ${choice} icon`}
                    aria-pressed={icon === choice}
                    onClick={() => setIcon(choice)}
                    className={`flex h-11 w-11 items-center justify-center rounded-xl border text-xl focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary ${icon === choice ? "border-primary bg-primary/10" : "border-line bg-paper"}`}
                  >
                    {choice}
                  </button>
                ))}
              </div>
            </fieldset>
            <fieldset className="mt-4">
              <legend className="text-sm font-medium text-ink">Choose a color</legend>
              <div className="mt-2 flex flex-wrap gap-3" role="group" aria-label="Category color">
                {colorOptions.map((choice) => (
                  <button
                    key={choice}
                    type="button"
                    aria-label={`Use color ${choice}`}
                    aria-pressed={color === choice}
                    onClick={() => setColor(choice)}
                    className={`h-11 w-11 rounded-full border-2 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${color === choice ? "border-ink" : "border-transparent"}`}
                    style={{ backgroundColor: choice }}
                  />
                ))}
              </div>
            </fieldset>
            <div className="mt-5 flex gap-2">
              <button type="button" onClick={resetForm} disabled={busy} className="min-h-11 flex-1 rounded-xl border border-line-strong px-4 text-sm font-semibold text-ink disabled:opacity-50">Cancel</button>
              <button type="submit" disabled={busy || !name.trim()} className="min-h-11 flex-1 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-ink disabled:opacity-50">{busy ? "Saving…" : editing ? "Save changes" : "Create category"}</button>
            </div>
          </form>
        ) : null}

        <ul className="mt-2 divide-y divide-line">
          {visible.map((category) => (
            <li key={category.id} className="flex min-h-[4.25rem] items-center gap-3 py-2">
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-lg" style={{ backgroundColor: `${category.color}20` }} aria-hidden="true">{category.icon}</span>
              <span className="min-w-0 flex-1 truncate text-sm font-medium text-ink">{category.name}</span>
              {archiveArmed === category.id ? (
                <>
                  <button type="button" disabled={busy} onClick={() => void archive(category)} className="min-h-11 rounded-lg px-2 text-xs font-semibold text-brick disabled:opacity-50">Confirm</button>
                  <button type="button" disabled={busy} onClick={() => setArchiveArmed(null)} className="min-h-11 rounded-lg px-2 text-xs font-semibold text-ink-soft">Cancel</button>
                </>
              ) : (
                <>
                  <button type="button" disabled={busy} onClick={() => startEdit(category)} aria-label={`Edit ${category.name}`} className="min-h-11 rounded-lg px-2 text-xs font-semibold text-primary disabled:opacity-50">Edit</button>
                  <button type="button" disabled={busy} onClick={() => void archive(category)} aria-label={`Archive ${category.name}`} className="min-h-11 rounded-lg px-2 text-xs font-semibold text-ink-soft disabled:opacity-50">Archive</button>
                </>
              )}
            </li>
          ))}
        </ul>
        {!loading && visible.length === 0 ? <p className="mt-3 rounded-xl border border-dashed border-line-strong px-4 py-5 text-center text-sm text-ink-soft">No active {kind} categories yet. Add one to use it in new entries.</p> : null}
      </section>

      {archived.length > 0 ? (
        <details className="mt-5 rounded-2xl border border-line bg-paper-raised px-4">
          <summary className="flex min-h-12 cursor-pointer items-center justify-between text-sm font-medium text-ink">Archived · {archived.length}</summary>
          <ul className="divide-y divide-line pb-2">
            {archived.map((category) => (
              <li key={category.id} className="flex min-h-[4.25rem] items-center gap-3 py-2">
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-lg opacity-60" style={{ backgroundColor: `${category.color}20` }} aria-hidden="true">{category.icon}</span>
                <span className="min-w-0 flex-1 truncate text-sm text-ink-soft">{category.name}</span>
                <button type="button" disabled={busy} onClick={() => void restore(category)} className="min-h-11 rounded-lg px-2 text-xs font-semibold text-primary disabled:opacity-50">Restore</button>
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </div>
  );
}
