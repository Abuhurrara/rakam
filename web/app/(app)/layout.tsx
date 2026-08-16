import { redirect } from "next/navigation";
import { checkAuth } from "@/lib/server-api";
import { AppShell } from "@/components/AppShell";

/**
 * Layer 2 of route protection, and the reason there is no login-page flash.
 *
 * This runs on the server before a byte of HTML is sent, so the browser never
 * paints an app shell it then has to bounce away from, and never paints a
 * login form that then disappears. A client-side check cannot do this — it
 * has to render something while its request is in flight, and that something
 * is the flash.
 *
 * The three-way result matters: "provisional" means the API was unreachable,
 * which is not the same as logged out. Sending a user to /login because the
 * free-tier server was asleep would be wrong, so that case renders the shell
 * with data gated off and lets the client settle it.
 */
export default async function AppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const auth = await checkAuth();

  if (auth.state === "unauthenticated") {
    redirect("/login");
  }

  return <AppShell initialAuth={auth}>{children}</AppShell>;
}
