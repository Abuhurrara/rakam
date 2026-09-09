import { AppShell } from "@/components/AppShell";

// This shell contains no user data. Middleware checks cookie presence; the
// browser verifies the session before mounting data screens. Never put a
// backend request here: it would delay route rendering and prefetching.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
