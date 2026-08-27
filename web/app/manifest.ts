import type { MetadataRoute } from "next";

/**
 * Served at /manifest.webmanifest, which middleware.ts already lets through
 * unauthenticated — the phone reads this before anyone has logged in.
 *
 * Two icon purposes on purpose. Android crops a `maskable` icon to whatever
 * shape the launcher uses, so that art keeps the page well inside the centre
 * 80% and lets the green bleed to the edge. The `any` art is never cropped,
 * so it carries its own rounded tile.
 */
export const dynamic = "force-static";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Rakam",
    short_name: "Rakam",
    description: "A private expense and lend/borrow ledger.",
    start_url: "/",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    // Paper, matching the light-mode --paper token. This is the splash screen
    // behind the icon while the app boots, so anything else flashes.
    background_color: "#f6f1e7",
    theme_color: "#f6f1e7",
    icons: [
      {
        src: "/icons/icon-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/icons/icon-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/icons/icon-maskable-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "maskable",
      },
      {
        src: "/icons/icon-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
