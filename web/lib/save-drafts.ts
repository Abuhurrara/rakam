import type { TransactionInput } from "./types";

export type SaveDraft = {
  version: 1;
  key: string;
  input: TransactionInput;
  transactionID?: string;
};

const prefix = (userID: string) => `rakam.saves.v1.${userID}.`;

export function storeDraft(
  storage: Storage,
  userID: string,
  draft: SaveDraft,
): void {
  storage.setItem(prefix(userID) + draft.key, JSON.stringify(draft));
}

export function removeDraft(
  storage: Storage,
  userID: string,
  key: string,
): void {
  storage.removeItem(prefix(userID) + key);
}

export function readDrafts(storage: Storage, userID: string): SaveDraft[] {
  const drafts: SaveDraft[] = [];
  for (let i = 0; i < storage.length; i++) {
    const key = storage.key(i);
    if (!key?.startsWith(prefix(userID))) continue;
    const value: unknown = JSON.parse(storage.getItem(key) ?? "null");
    if (!isDraft(value) || key !== prefix(userID) + value.key) {
      throw new Error(
        "A saved draft could not be read. Your stored data has been kept.",
      );
    }
    drafts.push(value);
  }
  return drafts;
}

function isDraft(value: unknown): value is SaveDraft {
  if (!value || typeof value !== "object") return false;
  const d = value as Partial<SaveDraft>;
  const input = d.input;
  return (
    d.version === 1 &&
    typeof d.key === "string" &&
    (d.transactionID === undefined || typeof d.transactionID === "string") &&
    !!input &&
    (input.kind === "expense" || input.kind === "income") &&
    typeof input.amount === "string" &&
    typeof input.occurred_at === "string" &&
    (input.category_id === null || typeof input.category_id === "string") &&
    (input.description === null || typeof input.description === "string")
  );
}
