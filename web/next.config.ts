import type { NextConfig } from "next";

// The browser must only ever talk to this origin. Every API call from the
// client is a relative /api/... path that this rewrite proxies to the Go
// service, which is what makes the session cookie same-origin and CORS
// unnecessary. See SPEC.md "Killing CORS".
//
// API_ORIGIN is deliberately not NEXT_PUBLIC_ — it must never reach the
// browser bundle. It is read here and in lib/server-api.ts, both server-only.
const apiOrigin = process.env.API_ORIGIN;

if (!apiOrigin) {
  throw new Error(
    "API_ORIGIN is not set. Copy web/.env.example to web/.env.local and point it at the Go API (e.g. http://localhost:8091).",
  );
}

const nextConfig: NextConfig = {
  // Lets a production build go somewhere other than .next, so building does
  // not clobber a dev server that is already running against it.
  distDir: process.env.NEXT_DIST_DIR || ".next",

  async rewrites() {
    return [
      {
        // The Go routes are registered with the /api prefix already
        // (httpapi/router.go), so the destination keeps it.
        source: "/api/:path*",
        destination: `${apiOrigin}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
