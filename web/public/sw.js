/*
 * Rakam service worker. Hand-written — SPEC.md allows no extra npm packages,
 * and the caching rules here are short enough that a library would hide more
 * than it helps.
 *
 * The one rule that matters: THIS FILE NEVER CACHES ANYTHING USER-SPECIFIC.
 *
 * Money data and the pages that render it stay on the network, always. A
 * cached balance is a wrong balance, and a cached authenticated page would
 * outlive the session cookie that was supposed to gate it. So:
 *
 *   /api/*            never touched. Straight to the network, every time.
 *   navigations       network only. If the network is gone, show /offline
 *                     instead of the browser's dinosaur. Nothing is stored.
 *   /_next/static/*   cache-first. Hashed, immutable, no user data in it.
 *   /icons/*          cache-first. Static art.
 *
 * That is enough to make the installed app open instantly and survive being
 * offline without ever showing stale or someone else's numbers.
 *
 * Bump VERSION to retire every old cache on the next activate.
 */

const VERSION = "rakam-v1";
const SHELL_CACHE = `${VERSION}-shell`;
const ASSET_CACHE = `${VERSION}-assets`;
const OFFLINE_URL = "/offline";

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(SHELL_CACHE);
      // Only the offline fallback. It is a static page with no user data,
      // and middleware.ts lets it through without a session.
      await cache.add(new Request(OFFLINE_URL, { cache: "reload" }));
      await self.skipWaiting();
    })(),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(
        keys
          .filter((key) => !key.startsWith(VERSION))
          .map((key) => caches.delete(key)),
      );
      await self.clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  const request = event.request;

  // Writes are never cacheable, and a POST must not be replayed from here.
  if (request.method !== "GET") return;

  const url = new URL(request.url);

  // Someone else's origin is not ours to cache.
  if (url.origin !== self.location.origin) return;

  // The API is the source of truth for money. Never intercept it — not even
  // to fall back offline. lib/api.ts already turns a failed fetch into a
  // retry toast, which is the honest thing to show.
  if (url.pathname.startsWith("/api/")) return;

  if (
    url.pathname.startsWith("/_next/static/") ||
    url.pathname.startsWith("/icons/")
  ) {
    event.respondWith(cacheFirst(request));
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(networkThenOffline(request));
  }
});

/** Immutable assets only. Safe to keep forever; nothing personal inside. */
async function cacheFirst(request) {
  const cache = await caches.open(ASSET_CACHE);
  const hit = await cache.match(request);
  if (hit) return hit;

  const response = await fetch(request);
  // Opaque and error responses are not worth storing.
  if (response.ok && response.type === "basic") {
    cache.put(request, response.clone());
  }
  return response;
}

/**
 * Always ask the network. The response is deliberately not cached — it may be
 * an authenticated page. Losing the network shows the offline page instead of
 * a browser error, which is all SPEC.md asks for.
 */
async function networkThenOffline(request) {
  try {
    return await fetch(request);
  } catch {
    const cache = await caches.open(SHELL_CACHE);
    const offline = await cache.match(OFFLINE_URL);
    return (
      offline ??
      new Response("You are offline.", {
        status: 503,
        headers: { "Content-Type": "text/plain; charset=utf-8" },
      })
    );
  }
}
