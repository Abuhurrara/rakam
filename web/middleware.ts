import { NextResponse, type NextRequest } from "next/server";

/**
 * Layer 1 of route protection: is there a session cookie at all?
 *
 * Middleware runs on the server, so it can read the httpOnly cookie that JS
 * cannot. When the cookie is missing entirely — the common logged-out case —
 * this redirects with zero network calls and zero rendering, so the user
 * never sees a frame of the app shell.
 *
 * It deliberately does not try to validate the token. That needs the API (or
 * the JWT secret, which has no business being in the web app), and is layer
 * 2's job in app/(app)/layout.tsx.
 */

const SESSION_COOKIE = "rakam_session";

export function middleware(req: NextRequest) {
  const hasSession = req.cookies.has(SESSION_COOKIE);
  const { pathname } = req.nextUrl;

  if (!hasSession && pathname !== "/login") {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    url.search = "";
    return NextResponse.redirect(url);
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    /*
     * Everything except:
     *   api/*      — proxied to the Go service by the rewrite; the API does
     *                its own auth and returns a real 401
     *   _next/*    — build output
     *   sw.js      — the service worker. The browser fetches it with no
     *                session on a first visit; redirecting it to /login
     *                would register the login page as the worker script and
     *                the install would fail outright.
     *   offline    — the worker precaches this at install time, which also
     *                happens before anyone has logged in. It holds no data.
     *   static files and the PWA manifest
     */
    "/((?!api|_next/static|_next/image|favicon.ico|manifest.webmanifest|sw\\.js|offline|icons/|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico)$).*)",
  ],
};
