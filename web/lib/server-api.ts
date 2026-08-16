import { headers } from "next/headers";
import type { User } from "./types";

/**
 * Server-side half of route protection.
 *
 * This runs during rendering, before a single byte of HTML reaches the
 * browser, which is the whole reason there is no login-page flash: the
 * authenticated-or-not decision is already made by the time anything paints.
 *
 * Unlike the browser client, this talks to API_ORIGIN directly rather than
 * through the /api rewrite — a server-side fetch has no cookie jar and needs
 * an absolute URL. API_ORIGIN stays server-only either way.
 */

export const SESSION_COOKIE = "rakam_session";

/**
 * Deliberately short. A cold Render instance can take the better part of a
 * minute, and blocking HTML that long would give the user a white screen —
 * exactly what requirement 3 forbids. When this budget is blown we hand off
 * to the client, which waits properly with a "waking up" screen.
 */
const SERVER_AUTH_TIMEOUT_MS = 3_000;

export type AuthCheck =
  /** The API confirmed the session. Safe to render data. */
  | { state: "authenticated"; user: User }
  /** The API rejected the session, or there is no cookie at all. */
  | { state: "unauthenticated" }
  /**
   * We could not reach the API in time, so we know nothing. Notably this is
   * NOT "logged out": redirecting to /login because the server was asleep
   * would be both wrong and the flash we are trying to avoid. The client
   * resolves it against the real API before any data is mounted.
   */
  | { state: "provisional"; reason: string };

export async function checkAuth(): Promise<AuthCheck> {
  const origin = process.env.API_ORIGIN;
  if (!origin) return { state: "provisional", reason: "API_ORIGIN is not set" };

  const cookie = (await headers()).get("cookie") ?? "";
  // No cookie at all means logged out for certain, with no network needed.
  // (middleware.ts normally catches this first; this is the belt to its
  // braces, and covers requests the matcher skips.)
  if (!cookie.includes(`${SESSION_COOKIE}=`)) {
    return { state: "unauthenticated" };
  }

  try {
    const res = await fetch(`${origin}/api/auth/me`, {
      headers: { cookie },
      cache: "no-store",
      signal: AbortSignal.timeout(SERVER_AUTH_TIMEOUT_MS),
    });

    // Any 4xx means this session will never work, so retrying is pointless.
    // 401 is a bad or expired token; 404 is a valid token naming a user that
    // no longer exists (the database was re-seeded under an old cookie). Both
    // are "log in again", not "wait and try again".
    if (res.status >= 400 && res.status < 500) {
      return { state: "unauthenticated" };
    }
    // 5xx is the server having a bad moment — that one is worth waiting out.
    if (!res.ok) {
      return { state: "provisional", reason: `API returned ${res.status}` };
    }
    return { state: "authenticated", user: (await res.json()) as User };
  } catch {
    return { state: "provisional", reason: "API unreachable" };
  }
}
