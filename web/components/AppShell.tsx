"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { ApiError, me, setUnauthorizedHandler } from "@/lib/api";
import { CategoriesProvider } from "./CategoriesProvider";
import { SavesProvider } from "./SavesProvider";
import { AddSheetProvider } from "./AddSheet";
import { ToastProvider } from "./Toast";
import { TabBar } from "./TabBar";
import { WakeScreen } from "./WakeScreen";

// The shared layout keeps this session across tab navigation. No private
// screen or draft mounts before /auth/me verifies the HttpOnly cookie.
/** After this many failed session checks, stop asking and tell the user. */
const MAX_SESSION_RETRIES = 5;

/**
 * The session check kept failing with something that should have been
 * temporary. Better a dead end the user can act on than a spinner that
 * quietly retries forever.
 */
function SessionStuck({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      role="alert"
      className="flex min-h-[70vh] items-center justify-center px-6"
    >
      <div className="max-w-xs rounded-2xl border border-line bg-paper-raised px-7 py-8 text-center">
        <p className="font-medium text-ink">Can&apos;t reach your account</p>
        <p className="mt-1.5 text-sm leading-relaxed text-ink-soft">
          We could not check your session. Try again, or sign in fresh.
        </p>
        <div className="mt-5 flex flex-col gap-2">
          <button
            type="button"
            onClick={onRetry}
            className="min-h-11 rounded-full bg-primary px-5 text-sm font-semibold text-primary-ink"
          >
            Try again
          </button>
          <a
            href="/login"
            className="flex min-h-11 items-center justify-center rounded-full border border-line-strong px-5 text-sm font-medium text-ink"
          >
            Sign in again
          </a>
        </div>
      </div>
    </div>
  );
}

export function AppShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [userID, setUserID] = useState<string | null>(null);
  const ready = userID !== null;
  const [stuck, setStuck] = useState(false);
  const [slow, setSlow] = useState(false);
  const [longWait, setLongWait] = useState(false);
  const [attemptID, setAttemptID] = useState(0);
  const retry = () => {
    setStuck(false);
    setSlow(false);
    setLongWait(false);
    setAttemptID((n) => n + 1);
  };

  // One place decides what an expired session does, for every call in the app.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      setUserID(null);
      router.replace("/login");
    });
    return () => setUnauthorizedHandler(null);
  }, [router]);

  // The session request itself wakes Render; do not wait for a health ping.
  useEffect(() => {
    if (ready) return;
    let cancelled = false;
    const controller = new AbortController();
    const slowTimer = setTimeout(() => setSlow(true), 2_000);
    const longTimer = setTimeout(() => setLongWait(true), 45_000);

    (async () => {
      for (let attempt = 0; !cancelled; attempt++) {
        try {
          const user = await me({
            skipAuthRedirect: true,
            signal: controller.signal,
          });
          if (cancelled) return;
          clearTimeout(slowTimer);
          clearTimeout(longTimer);
          setUserID(user.id);
          return;
        } catch (err) {
          if (cancelled) return;
          // A bad/expired session or a deleted account requires login.
          // Rate limits and temporary failures must not log the user out.
          if (
            err instanceof ApiError &&
            (err.status === 401 || err.status === 404)
          ) {
            router.replace("/login");
            return;
          }
          // A 5xx or a network blip is worth waiting out — but not forever.
          if (attempt >= MAX_SESSION_RETRIES) {
            if (!cancelled) setStuck(true);
            return;
          }
          await new Promise((r) => setTimeout(r, 2_000));
        }
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      clearTimeout(slowTimer);
      clearTimeout(longTimer);
    };
  }, [ready, attemptID, router]);

  return (
    <ToastProvider>
      <CategoriesProvider enabled={ready}>
        <SavesProvider key={userID ?? "waiting"} userID={userID}>
          <AddSheetProvider>
            {ready ? (
              <main className="mx-auto min-h-dvh max-w-lg pb-28">
                {children}
              </main>
            ) : (
              <main className="mx-auto min-h-dvh max-w-lg">
                {stuck ? (
                  <SessionStuck onRetry={retry} />
                ) : (
                  <WakeScreen
                    longWait={longWait}
                    description={
                      slow
                        ? undefined
                        : "Your account is being checked securely."
                    }
                    onRetry={retry}
                    message={
                      slow ? "Waking up the server" : "Checking your session"
                    }
                  />
                )}
              </main>
            )}
            <TabBar canAdd={ready} />
          </AddSheetProvider>
        </SavesProvider>
      </CategoriesProvider>
    </ToastProvider>
  );
}
