"use client";

import { Spinner } from "./Spinner";

/**
 * What the user sees while the API is cold, and while a provisional session
 * is being resolved. Never an error, never an empty screen.
 */
export function WakeScreen({
  longWait,
  onRetry,
  variant = "full",
  message,
}: {
  longWait?: boolean;
  onRetry?: () => void;
  /** "full" replaces the screen; "overlay" floats over mounted content. */
  variant?: "full" | "overlay";
  message?: string;
}) {
  const body = (
    <div className="mx-6 flex max-w-xs flex-col items-center gap-4 rounded-2xl border border-line bg-paper-raised px-7 py-8 text-center">
      <Spinner className="text-primary" size={28} />
      <div className="space-y-1.5">
        <p className="font-medium text-ink">
          {message ?? "Waking up the server"}
        </p>
        <p className="text-sm leading-relaxed text-ink-soft">
          {longWait
            ? "Still waking up. On the free tier this can take a minute."
            : "It sleeps when nobody is using it. This takes a few seconds."}
        </p>
      </div>
      {longWait && onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="min-h-11 rounded-full border border-line-strong px-5 text-sm font-medium text-ink transition-colors hover:bg-paper-sunken"
        >
          Try again
        </button>
      ) : null}
    </div>
  );

  if (variant === "overlay") {
    return (
      <div
        role="status"
        aria-live="polite"
        className="fixed inset-0 z-40 flex items-center justify-center bg-overlay backdrop-blur-[2px]"
      >
        {body}
      </div>
    );
  }

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex min-h-[70vh] items-center justify-center"
    >
      {body}
    </div>
  );
}
