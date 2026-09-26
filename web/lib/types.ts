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

export type CategoryInput = {
  name: string;
  kind: Kind;
  icon: string;
  color: string;
  sort_order: number;
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
  expense_paisa: number;
  limit: number;
  offset: number;
};

export type TransactionQuery = {
  kind?: Kind;
  month?: string;
  category_id?: string;
  q?: string;
  limit?: number;
  offset?: number;
};

/** api/internal/httpapi/person.go — personResponse */
export type Person = {
  id: string;
  name: string;
  phone: string | null;
  notes: string | null;
  balance_paisa: number;
  created_at: string;
  updated_at: string;
};

export type PersonInput = {
  name: string;
  phone: string | null;
  notes: string | null;
};

/** api/internal/httpapi/debt.go — debtEntryResponse */
export type DebtDirection = "i_owe" | "they_owe";

export type DebtEntry = {
  id: string;
  person_id: string;
  direction: DebtDirection;
  amount_paisa: number;
  description: string;
  incurred_at: string;
  settled_at: string | null;
  created_at: string;
  updated_at: string;
};

/** `amount` is the raw decimal string; the API parses it into paisa. */
export type DebtEntryInput = {
  direction: DebtDirection;
  amount: string;
  description: string;
  incurred_at: string;
};

export type SettlementInput = {
  create_transaction: boolean;
  category_id?: string | null;
};

export type Settlement = {
  debt_entry: DebtEntry;
  transaction: Transaction | null;
};

export type SettleAllResult = {
  debt_entries: DebtEntry[];
  transactions: Transaction[];
};

/** api/internal/httpapi/budget.go — budgetResponse */
export type Budget = {
  id: string;
  category_id: string;
  month: string;
  limit_paisa: number;
  created_at: string;
  updated_at: string;
};

/** api/internal/httpapi/budget.go — budgetWithSpentResponse */
export type BudgetWithSpent = {
  category: Category;
  budget: Budget | null;
  spent_paisa: number;
};

/** `limit` remains the raw decimal string typed by the user. */
export type BudgetInput = {
  category_id: string;
  month: string;
  limit: string;
};

/** api/internal/httpapi/bill.go — billResponse */
export type RecurringBill = {
  id: string;
  name: string;
  amount_paisa: number;
  category_id: string | null;
  day_of_month: number;
  is_active: boolean;
  last_generated_month: string | null;
  created_at: string;
  updated_at: string;
};

/** api/internal/httpapi/summary.go — upcomingBillResponse */
export type UpcomingBill = {
  bill: RecurringBill;
  due_at: string;
};

/** api/internal/httpapi/summary.go — summaryResponse */
export type Summary = {
  month: string;
  income_paisa: number;
  expense_paisa: number;
  net_paisa: number;
  days_remaining: number;
  budget_limit_paisa: number;
  budget_spent_paisa: number;
  owed_to_me_paisa: number;
  i_owe_paisa: number;
  net_owed_paisa: number;
  upcoming_bills: UpcomingBill[];
  recent_transactions: Transaction[];
};
