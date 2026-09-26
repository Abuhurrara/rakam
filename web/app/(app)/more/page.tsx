import { ChangePasswordForm } from "@/components/ChangePasswordForm";
import { ExportButton } from "@/components/ExportButton";
import { SignOutButton } from "@/components/SignOutButton";
import Link from "next/link";

export default function MorePage() {
  return (
    <>
      <div className="px-4 py-6">
        <h1 className="text-xl font-semibold text-ink">More</h1>
        <ChangePasswordForm />
        <Link
          href="/categories"
          className="mt-3 flex min-h-14 items-center justify-between rounded-2xl border border-line bg-paper-raised px-4 text-sm font-medium text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          <span>Manage categories</span>
          <span aria-hidden="true" className="text-xl text-ink-faint">›</span>
        </Link>
        <div className="mt-5">
          <ExportButton />
        </div>
        <div className="mt-3">
          <SignOutButton />
        </div>
      </div>
    </>
  );
}
