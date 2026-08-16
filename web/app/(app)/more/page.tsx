import { PhaseStub } from "@/components/PhaseStub";
import { SignOutButton } from "@/components/SignOutButton";

export default function MorePage() {
  return (
    <>
      <PhaseStub
        title="More"
        summary="Recurring bills with next due dates, categories management, the work log, and a full JSON export."
      />
      <div className="px-4">
        <SignOutButton />
      </div>
    </>
  );
}
