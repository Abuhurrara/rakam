import { PersonLedgerScreen } from "@/components/PersonLedgerScreen";

// This server wrapper resolves only the public route parameter. Private data
// remains client-fetched after AppShell verifies the session.
export default async function PersonLedgerPage({
  params,
}: {
  params: Promise<{ personId: string }>;
}) {
  const { personId } = await params;
  return <PersonLedgerScreen personID={personId} />;
}
