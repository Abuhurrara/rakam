/**
 * Wire types, mirroring the JSON tags on the Go response structs exactly.
 * If one of these drifts from the API, the app breaks silently — so each is
 * annotated with the file it came from.
 */

/** api/internal/httpapi/auth.go — userResponse */
export type User = {
  id: string;
  email: string;
  name: string;
};

export type Kind = "expense" | "income";

/** api/internal/httpapi/category.go — categoryResponse */
export type Category = {
  id: string;
  name: string;
  kind: Kind;
  icon: string;
  color: string;
  sort_order: number;
  is_archived: boolean;
  created_at: string;
  updated_at: string;
};

/**
 * api/internal/httpapi/transaction.go — transactionResponse.
 * Note `amount_paisa`: an integer, never a formatted string.
 */
export type Transaction = {
  id: string;
  kind: Kind;
  amount_paisa: number;
  category_id: string | null;
  description: string | null;
  occurred_at: string;
  recurring_bill_id: string | null;
  created_at: string;
  updated_at: string;
};

/**
 * api/internal/httpapi/transaction.go — transactionRequest.
 *
 * `amount` is a STRING: the raw text the user typed. The Go side parses it
 * with domain.ParseMoney. Never send a number, and never convert here.
 *
 * Every field is required on PATCH as well as POST — the API's PATCH runs the
 * same decoder as POST and replaces the whole row, so a partial body would
 * null out the fields it omits.
 */
export type TransactionInput = {
  kind: Kind;
  amount: string;
  category_id: string | null;
  description: string | null;
  occurred_at: string;
};

/** api/internal/httpapi/transaction.go — listTransactionsResponse */
export type TransactionList = {
  transactions: Transaction[];
  total: number;
  limit: number;
  offset: number;
};

export type TransactionQuery = {
  month?: string;
  category_id?: string;
  q?: string;
  limit?: number;
  offset?: number;
};
