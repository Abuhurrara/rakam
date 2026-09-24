"use client";

import { useState, type FormEvent } from "react";
import { ApiError, changePassword } from "@/lib/api";

export function ChangePasswordForm() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setMessage("");
    if (newPassword !== confirmPassword) {
      setError("The new passwords do not match.");
      return;
    }
    if (newPassword.length < 12 || new TextEncoder().encode(newPassword).length > 72) {
      setError("Use a password between 12 and 72 bytes.");
      return;
    }

    setBusy(true);
    try {
      await changePassword(currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setMessage("Password changed. Other signed-in devices will need to sign in again.");
    } catch (cause) {
      setError(
        cause instanceof ApiError && cause.status === 400
          ? "Your current password is incorrect."
          : "Could not change your password. Please try again.",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="mt-5 rounded-2xl border border-line bg-paper-raised px-4 py-4">
      <h2 className="text-base font-semibold text-ink">Change password</h2>
      <p className="mt-1 text-sm leading-relaxed text-ink-soft">
        Use at least 12 characters. Changing it signs out your other devices.
      </p>
      <form className="mt-4 space-y-3" onSubmit={submit}>
        <label className="block text-sm font-medium text-ink">
          Current password
          <input
            autoComplete="current-password"
            className="mt-1 min-h-12 w-full rounded-xl border border-line-strong bg-paper px-3 text-base"
            type="password"
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
            required
          />
        </label>
        <label className="block text-sm font-medium text-ink">
          New password
          <input
            autoComplete="new-password"
            className="mt-1 min-h-12 w-full rounded-xl border border-line-strong bg-paper px-3 text-base"
            type="password"
            value={newPassword}
            onChange={(event) => setNewPassword(event.target.value)}
            required
            minLength={12}
          />
        </label>
        <label className="block text-sm font-medium text-ink">
          Confirm new password
          <input
            autoComplete="new-password"
            className="mt-1 min-h-12 w-full rounded-xl border border-line-strong bg-paper px-3 text-base"
            type="password"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            required
            minLength={12}
          />
        </label>
        {error ? <p className="text-sm text-brick" role="alert">{error}</p> : null}
        {message ? <p className="text-sm text-primary" role="status">{message}</p> : null}
        <button
          className="min-h-12 w-full rounded-full bg-primary px-5 text-sm font-semibold text-white disabled:opacity-50"
          type="submit"
          disabled={busy || !currentPassword || !newPassword || !confirmPassword}
        >
          {busy ? "Changing…" : "Update password"}
        </button>
      </form>
    </section>
  );
}
