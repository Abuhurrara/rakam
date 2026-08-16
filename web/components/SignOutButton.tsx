"use client";

import { logout } from "@/lib/api";
import { friendlyMessage, useMutation } from "@/lib/useMutation";
import { Spinner } from "./Spinner";
import { useToast } from "./Toast";

export function SignOutButton() {
  const toast = useToast();
  const signOut = useMutation(async () => logout());

  return (
    <button
      type="button"
      disabled={signOut.status === "pending"}
      onClick={async () => {
        const res = await signOut.run(undefined);
        if (res.ok) {
          // A full navigation, so middleware sees the cleared cookie and the
          // in-memory category cache goes with it.
          window.location.href = "/login";
        } else {
          toast.show({
            kind: "error",
            message: "Couldn't sign out",
            detail: friendlyMessage(res.error),
            action: { label: "Retry", onClick: () => void signOut.retry() },
          });
        }
      }}
      className="flex min-h-12 w-full items-center justify-center gap-2 rounded-2xl border border-line text-sm font-medium text-brick disabled:opacity-50"
    >
      {signOut.status === "pending" ? <Spinner size={16} /> : null}
      Sign out
    </button>
  );
}
