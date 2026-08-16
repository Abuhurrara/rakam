/**
 * An honest placeholder. The tab is real and routes correctly; the screen
 * behind it is phase 7. Better than a half-built version of the real thing.
 */
export function PhaseStub({
  title,
  summary,
}: {
  title: string;
  summary: string;
}) {
  return (
    <div className="px-4 py-8">
      <h1 className="text-xl font-semibold text-ink">{title}</h1>
      <div className="mt-6 rounded-2xl border border-dashed border-line-strong bg-paper-raised px-5 py-6">
        <p className="text-label uppercase tracking-widest text-gold">
          Coming in phase 7
        </p>
        <p className="mt-2 text-sm leading-relaxed text-ink-soft">{summary}</p>
      </div>
    </div>
  );
}
