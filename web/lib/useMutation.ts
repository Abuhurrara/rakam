"use client";

import { useCallback, useRef, useState } from "react";
import { ApiError } from "./api";

/**
 * Every mutation in the app goes through this hook. That is the point: it
 * makes "a save that silently failed" structurally impossible rather than
 * something each screen has to remember to handle.
 *
 * Three states, all of which the UI is expected to show:
 *   pending — something is in flight
 *   success — it landed
 *   error   — it did not, and `retry()` will try the identical payload again
 */

export type MutationStatus = "idle" | "pending" | "success" | "error";

export type MutationResult<R> =
  | { ok: true; data: R }
  | { ok: false; error: ApiError };

export function useMutation<A, R>(fn: (arg: A) => Promise<R>) {
  const [status, setStatus] = useState<MutationStatus>("idle");
  const [error, setError] = useState<ApiError | null>(null);
  const lastArg = useRef<A | null>(null);
  // Guards against a slow first attempt resolving after a retry and
  // overwriting the newer result.
  const runId = useRef(0);

  const run = useCallback(
    async (arg: A): Promise<MutationResult<R>> => {
      const id = ++runId.current;
      lastArg.current = arg;
      setStatus("pending");
      setError(null);

      try {
        const data = await fn(arg);
        if (runId.current === id) {
          setStatus("success");
        }
        return { ok: true, data };
      } catch (err) {
        const apiErr =
          err instanceof ApiError
            ? err
            : new ApiError(
                err instanceof Error ? err.message : "Something went wrong.",
              );
        if (runId.current === id) {
          setStatus("error");
          setError(apiErr);
        }
        return { ok: false, error: apiErr };
      }
    },
    [fn],
  );

  /** Re-runs the last payload byte for byte. */
  const retry = useCallback((): Promise<MutationResult<R>> | undefined => {
    if (lastArg.current === null) return undefined;
    return run(lastArg.current);
  }, [run]);

  const reset = useCallback(() => {
    runId.current++;
    setStatus("idle");
    setError(null);
  }, []);

  return { status, error, run, retry, reset } as const;
}

/** Turns an ApiError into something worth showing a person. */
export function friendlyMessage(err: ApiError): string {
  if (err.isOffline) return "You're offline.";
  if (err.isTimeout) return "The server took too long.";
  if (err.status === 401) return "Your session expired.";
  if (err.status === 404) return "That's not there any more.";
  if (err.status >= 500) return "The server had a problem.";
  if (err.status >= 400) return err.message;
  return "Could not reach the server.";
}
