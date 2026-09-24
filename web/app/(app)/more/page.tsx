import { ChangePasswordForm } from "@/components/ChangePasswordForm";
import { ExportButton } from "@/components/ExportButton";
import { SignOutButton } from "@/components/SignOutButton";

export default function MorePage() {
  return (
    <>
      <div className="px-4 py-6">
        <h1 className="text-xl font-semibold text-ink">More</h1>
        <ChangePasswordForm />
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
