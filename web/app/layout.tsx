import type { Metadata, Viewport } from "next";
import { IBM_Plex_Sans, IBM_Plex_Mono } from "next/font/google";
import { ServiceWorkerRegistrar } from "@/components/ServiceWorkerRegistrar";
import "./globals.css";

// Humanist sans for body, tabular monospace for every number.
const plexSans = IBM_Plex_Sans({
  variable: "--font-plex-sans",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  display: "swap",
});

const plexMono = IBM_Plex_Mono({
  variable: "--font-plex-mono",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "Rakam",
  description: "A private expense and lend/borrow ledger.",
  // The <link rel="manifest"> tag comes from app/manifest.ts automatically.
  // iOS ignores manifest icons on the home screen and reads only this one.
  icons: {
    apple: "/icons/apple-touch-icon.png",
  },
  appleWebApp: {
    capable: true,
    title: "Rakam",
    // "default" keeps the status bar its own strip. "black-translucent" would
    // slide the page under the clock, which this fixed shell is not laid out
    // for even with viewportFit: "cover".
    statusBarStyle: "default",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  // The app is a fixed-shell mobile UI; zooming breaks the tab bar and the
  // keypad. Accessibility note: text still scales with the OS text-size
  // setting, which is the setting people actually use on a phone.
  maximumScale: 1,
  viewportFit: "cover",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f6f1e7" },
    { media: "(prefers-color-scheme: dark)", color: "#1a1713" },
  ],
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body className={`${plexSans.variable} ${plexMono.variable}`}>
        {/*
          Written by hand, not via metadata.appleWebApp. Next 15 emits only
          the standardised <meta name="mobile-web-app-capable"> and drops this
          legacy name, but iOS did not read the manifest's `display` member
          until 16.4 — before that, this tag is the only thing that opens the
          app in a standalone window instead of inside Safari. React 19 hoists
          it into <head> from here.
        */}
        <meta name="apple-mobile-web-app-capable" content="yes" />
        {children}
        <ServiceWorkerRegistrar />
      </body>
    </html>
  );
}
