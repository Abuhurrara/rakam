"use client";

import { useState, type FormEvent } from "react";
import { login } from "@/lib/api";
import { friendlyMessage, useMutation } from "@/lib/useMutation";
import { useServerWake } from "@/lib/useServerWake";
import { Spinner } from "./Spinner";

export function LoginForm() {
  const wake = useServerWake();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const signIn = useMutation(async (creds: { email: string; password: string }) =>
    login(creds.email, creds.password),
  );

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const res = await signIn.run({ email, password });
    if (res.ok) {
      // Full navigation rather than router.push: the session cookie was just
      // set, and this makes the server-side check in app/(app)/layout.tsx run
      // against it cleanly.
      window.location.href = "/";
    }
  }

  const pending = signIn.status === "pending";

  return (
    <main className="mx-auto flex min-h-dvh max-w-sm flex-col justify-center px-6 py-10">
      <header className="mb-8">
        <h1 className="text-3xl font-semibold tracking-tight text-ink">Rakam</h1>
        <p className="mt-1.5 text-sm text-ink-soft">
          Your expenses and your ledger, in one place.
        </p>
      </header>

      {wake.showWaking && !wake.awake ? (
        <div
          role="status"
          className="mb-5 flex items-center gap-3 rounded-xl border border-line bg-paper-raised px-4 py-3"
        >
          <Spinner size={16} className="text-primary" />
          <p className="text-sm text-ink-soft">
            {wake.longWait
              ? "Still waking up the server. This can take a minute."
              : "Waking up the server…"}
          </p>
        </div>
      ) : null}

      <form onSubmit={onSubmit} className="space-y-3">
        <div>
          <label
            htmlFor="email"
            className="text-label uppercase tracking-widest text-ink-faint"
          >
            Email
          </label>
          <input
            id="email"
            type="email"
            autoComplete="username"
            inputMode="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-1 min-h-12 w-full rounded-xl border border-line bg-paper-raised px-3.5 text-ink focus:border-primary focus:outline-none"
          />
        </div>

        <div>
          <label
            htmlFor="password"
            className="text-label uppercase tracking-widest text-ink-faint"
          >
            Password
          </label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 min-h-12 w-full rounded-xl border border-line bg-paper-raised px-3.5 text-ink focus:border-primary focus:outline-none"
          />
        </div>

        {signIn.status === "error" && signIn.error ? (
          <p
            role="alert"
            className="rounded-xl border border-brick/30 bg-brick-tint px-3.5 py-2.5 text-sm text-brick"
          >
            {signIn.error.status === 401
              ? "That email and password don't match."
              : friendlyMessage(signIn.error)}
          </p>
        ) : null}

        <button
          type="submit"
          disabled={pending}
          className="flex min-h-[3.25rem] w-full items-center justify-center gap-2 rounded-2xl bg-primary text-base font-semibold text-primary-ink disabled:opacity-60"
        >
          {pending ? <Spinner size={18} /> : null}
          {pending ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </main>
  );
}
