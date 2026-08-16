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
      while (!cancelled) {
        try {
          await me({ skipAuthRedirect: true });
          if (!cancelled) setReady(true);
          return;
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            router.replace("/login");
            return;
          }
          // Reachable but unhappy (a 500, a blip). Wait and ask again rather
          // than guessing about the session.
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
                <WakeScreen
                  longWait={wake.longWait}
                  onRetry={wake.retry}
                  message={
                    wake.awake ? "Checking your session" : "Waking up the server"
                  }
                />
              </main>
            )}
          </AddSheetProvider>
        </SavesProvider>
      </CategoriesProvider>
    </ToastProvider>
  );
}
