import { redirect } from "next/navigation";
import { checkAuth } from "@/lib/server-api";
import { LoginForm } from "@/components/LoginForm";

/**
 * Checked on the server so an already-signed-in user never sees this form
 * flash before being sent on. A provisional result (API unreachable) falls
 * through to the form — the user can still type while the server wakes, and
 * the form's own health banner tells them what is happening.
 */
export default async function LoginPage() {
  const auth = await checkAuth();
  if (auth.state === "authenticated") redirect("/");
  return <LoginForm />;
}
