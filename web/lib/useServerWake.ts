"use client";

import { useCallback, useEffect, useState } from "react";
import { health } from "./api";

/**
 * The API sleeps on Render's free tier. A cold start is a normal, expected
 * state — not an error — so this polls /api/health until the server answers
 * and reports how long it has been waiting, so the UI can say so.
 *
 * Requirement: if health has not resolved within 2 seconds, the app shows a
 * "waking up the server" screen. Never an empty screen, never an error.
 */

const SHOW_WAKING_AFTER_MS = 2_000;
const SAY_STILL_WAITING_AFTER_MS = 45_000;
const BACKOFF_MS = [2_000, 4_000, 8_000];

export type ServerWake = {
  /** The server has answered /api/health. */
  awake: boolean;
  /** Show the "waking up" UI — health is slow but we are still trying. */
  showWaking: boolean;
  /** It has been long enough that the copy should acknowledge it. */
  longWait: boolean;
  /** Start the whole check again from scratch. */
  retry: () => void;
};

export function useServerWake(): ServerWake {
  const [awake, setAwake] = useState(false);
  const [showWaking, setShowWaking] = useState(false);
  const [longWait, setLongWait] = useState(false);
  const [nonce, setNonce] = useState(0);

  const retry = useCallback(() => {
    setAwake(false);
    setShowWaking(true);
    setLongWait(false);
    setNonce((n) => n + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    const showTimer = setTimeout(() => {
      if (!cancelled) setShowWaking(true);
    }, SHOW_WAKING_AFTER_MS);

    const longTimer = setTimeout(() => {
      if (!cancelled) setLongWait(true);
    }, SAY_STILL_WAITING_AFTER_MS);

    (async () => {
      for (let attempt = 0; !cancelled; attempt++) {
        try {
          const res = await health(controller.signal);
          if (cancelled) return;
          if (res?.status === "ok") {
            setAwake(true);
            setShowWaking(false);
            return;
          }
        } catch {
          // A refused connection, a timeout or a 503 all mean the same thing
          // here: not up yet. Keep waiting rather than surfacing a failure.
        }
        if (cancelled) return;
        const wait = BACKOFF_MS[Math.min(attempt, BACKOFF_MS.length - 1)];
        await new Promise((r) => setTimeout(r, wait));
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      clearTimeout(showTimer);
      clearTimeout(longTimer);
    };
  }, [nonce]);

  return { awake, showWaking, longWait, retry };
}
