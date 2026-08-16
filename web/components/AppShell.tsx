"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { ApiError, me, setUnauthorizedHandler } from "@/lib/api";
import { useServerWake } from "@/lib/useServerWake";
import type { AuthCheck } from "@/lib/server-api";
import { CategoriesProvider } from "./CategoriesProvider";
import { SavesProvider } from "./SavesProvider";
import { AddSheetProvider } from "./AddSheet";
import { ToastProvider } from "./Toast";
import { TabBar } from "./TabBar";
import { WakeScreen } from "./WakeScreen";

/**
 * The authenticated shell, and the client half of route protection.
 *
 * The server already decided in app/(app)/layout.tsx. When it came back
 * "provisional" — it could not reach the API within its short budget — this
 * component resolves the question against the real API before anything that
 * depends on data is allowed to exist.
 *
 * `children` is genuinely not mounted until `ready` is true. That is
 * deliberate: gating on a boolean here means no screen added later can leak
 * a request against an unverified session by forgetting to check first.
 */
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
          The server answered, but not with your session. Try again, or sign in
          fresh.
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

export function AppShell({
  initialAuth,
  children,
}: {
  initialAuth: AuthCheck;
  children: ReactNode;
}) {
  const router = useRouter();
  const wake = useServerWake();

  // Only the server's definite "yes" starts us ready. "provisional" does not.
  const [ready, setReady] = useState(initialAuth.state === "authenticated");
  // The session check kept failing with something retryable. Never spin
  // silently — say so and hand the user a button.
  const [stuck, setStuck] = useState(false);

  // One place decides what an expired session does, for every call in the app.
  useEffect(() => {
    setUnauthorizedHandler(() => router.replace("/login"));
    return () => setUnauthorizedHandler(null);
  }, [router]);

  // Resolve a provisional session once the server is answering again.
  useEffect(() => {
    if (ready || !wake.awake) return;
    let cancelled = false;

    (async () => {
      for (let attempt = 0; !cancelled; attempt++) {
        try {
          await me({ skipAuthRedirect: true });
          if (!cancelled) setReady(true);
          return;
        } catch (err) {
          // Any 4xx is a permanent verdict on this session and will never
          // recover: 401 is a bad or expired token, 404 is a valid token
          // naming a user that no longer exists. Retrying those just hammers
          // the API forever, so go to the login page instead.
          if (err instanceof ApiError && err.status >= 400 && err.status < 500) {
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
    };
  }, [ready, wake.awake, router]);

  return (
    <ToastProvider>
      <CategoriesProvider enabled={ready}>
        <SavesProvider>
          <AddSheetProvider>
            {ready ? (
              <>
                <main className="mx-auto min-h-dvh max-w-lg pb-28">
                  {children}
                </main>
                {/* Warm path: nothing. Slow path: an overlay over live
                    content, because the session is already confirmed. */}
                {wake.showWaking && !wake.awake ? (
                  <WakeScreen
                    variant="overlay"
                    longWait={wake.longWait}
                    onRetry={wake.retry}
                  />
                ) : null}
                <TabBar />
              </>
            ) : (
              <main className="mx-auto min-h-dvh max-w-lg">
                {stuck ? (
                  <SessionStuck
                    onRetry={() => {
                      setStuck(false);
                      wake.retry();
                    }}
                  />
                ) : (
                  <WakeScreen
                    longWait={wake.longWait}
                    onRetry={wake.retry}
                    message={
                      wake.awake
                        ? "Checking your session"
                        : "Waking up the server"
                    }
                  />
                )}
              </main>
            )}
          </AddSheetProvider>
        </SavesProvider>
      </CategoriesProvider>
    </ToastProvider>
  );
}
