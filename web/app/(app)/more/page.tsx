import { PhaseStub } from "@/components/PhaseStub";
import { ExportButton } from "@/components/ExportButton";
import { SignOutButton } from "@/components/SignOutButton";

export default function MorePage() {
  return (
    <>
      <PhaseStub
        title="More"
        summary="Recurring bill management, category settings and the work log are coming later. You can download your ledger below."
      />
      <div className="px-4">
        <ExportButton />
        <SignOutButton />
      </div>
    </>
  );
}
