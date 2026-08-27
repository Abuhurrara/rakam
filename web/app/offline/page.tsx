/**
 * The service worker precaches this page and serves it when a navigation
 * fails. It must stay static and public — it is fetched at install time,
 * possibly before anyone has logged in, so middleware.ts lets it through.
 *
 * No API call, no session read, nothing dynamic. If this page ever needs
 * data it stops being servable offline, which defeats the point.
 */
export const dynamic = "force-static";

export const metadata = {
  title: "Offline · Rakam",
};

export default function OfflinePage() {
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center px-6 text-center">
      <div className="rounded-2xl border border-dashed border-line-strong bg-paper-raised px-6 py-8">
        <p className="text-label uppercase tracking-widest text-gold">
          No connection
        </p>
        <h1 className="mt-3 text-xl font-semibold text-ink">
          You are offline
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-ink-soft">
          Rakam needs the network to read and save money. Nothing you had
          already saved is lost — reconnect and it will all be here.
        </p>
      </div>
    </main>
  );
}
