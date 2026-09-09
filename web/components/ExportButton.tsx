/** A native download preserves user activation in an installed Android PWA,
 * carries the same-origin session cookie, and preserves int64 JSON verbatim.
 * The API sends Content-Disposition and Cache-Control: no-store.
 */
export function ExportButton() {
  return (
    <section className="mb-5 rounded-2xl border border-line p-4">
      <h2 className="font-medium">Back up your ledger</h2>
      <p className="mt-1 text-sm text-ink-soft">
        Download your complete saved history, including archived categories,
        budgets, bills and settled debts. Unfinished local drafts are not
        included.
      </p>
      <a
        href="/api/export"
        download="rakam-export.json"
        className="mt-3 inline-flex min-h-11 items-center rounded-xl bg-primary px-4 text-primary-ink"
      >
        Download JSON backup
      </a>
    </section>
  );
}
