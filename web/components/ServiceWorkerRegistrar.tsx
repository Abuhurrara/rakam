"use client";

import { useEffect } from "react";

/**
 * Registers public/sw.js. Renders nothing.
 *
 * Development is deliberately excluded. A service worker in front of the
 * Turbopack dev server serves yesterday's chunks and makes every change look
 * like it did not apply — a whole afternoon lost to a bug that is not real.
 */
export function ServiceWorkerRegistrar() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "production") return;
    if (!("serviceWorker" in navigator)) return;

    navigator.serviceWorker.register("/sw.js").catch((err) => {
      // Not fatal — the app works fine without it, it just will not install
      // or open offline. Worth seeing in the console rather than swallowing.
      console.error("Service worker registration failed:", err);
    });
  }, []);

  return null;
}
