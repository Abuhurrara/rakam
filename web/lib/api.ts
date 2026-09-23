import type {
  Budget,
  BudgetInput,
  BudgetWithSpent,
  Category,
  DebtEntry,
  DebtEntryInput,
  Person,
  PersonInput,
  SettleAllResult,
  Settlement,
  SettlementInput,
  Transaction,
  TransactionInput,
  TransactionList,
  TransactionQuery,
  Summary,
  User,
} from "./types";

/**
 * The one typed API client. No data-fetching library, per SPEC.md.
 *
 * Every call is a relative /api/... path. next.config.ts rewrites those to
 * the Go service, so the browser only ever talks to its own origin — which is
 * what makes the session cookie same-origin and CORS unnecessary.
 */

/**
 * The API runs on Render's free tier, where a sleeping instance can take the
 * better part of a minute to answer its first request. A cold start must
 * never surface to the user as a failure, so the timeout is generous. The
 * "waking up the server" screen (components/ServerWake.tsx) is what fills
 * that time visually.
 */
export const REQUEST_TIMEOUT_MS = 90_000;

export class ApiError extends Error {
  readonly status: number;
  readonly isTimeout: boolean;
  readonly isOffline: boolean;

  constructor(
    message: string,
    opts: { status?: number; isTimeout?: boolean; isOffline?: boolean } = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.status = opts.status ?? 0;
    this.isTimeout = opts.isTimeout ?? false;
    this.isOffline = opts.isOffline ?? false;
  }

  /** True when the request never got a verdict from the server. */
  get isNetworkish(): boolean {
    return this.status === 0;
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
  /**
   * Login expects a 401 for bad credentials and handles it inline, so it opts
   * out of the global session-expired redirect.
   */
  skipAuthRedirect?: boolean;
  timeoutMs?: number;
  idempotencyKey?: string;
};

/**
 * One place decides what an expired session does, so no caller repeats it.
 * The app layout registers a handler that routes to /login.
 */
let onUnauthorized: (() => void) | null = null;

export function setUnauthorizedHandler(fn: (() => void) | null): void {
  onUnauthorized = fn;
}

export async function apiFetch<T>(
  path: string,
  opts: RequestOptions = {},
): Promise<T> {
  const { method = "GET", body, skipAuthRedirect } = opts;

  const timeout = AbortSignal.timeout(opts.timeoutMs ?? REQUEST_TIMEOUT_MS);
  const signal = opts.signal
    ? AbortSignal.any([timeout, opts.signal])
    : timeout;

  let res: Response;
  try {
    res = await fetch(path, {
      method,
      signal,
      // The session cookie is httpOnly; this is what sends it.
      credentials: "include",
      headers: {
        ...(body ? { "Content-Type": "application/json" } : {}),
        ...(opts.idempotencyKey
          ? { "Idempotency-Key": opts.idempotencyKey }
          : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch (err) {
    throw toNetworkError(err);
  }

  // Read as text first. Two responses from this API have no body at all:
  // every DELETE and logout return 204, and the panic-recovery middleware
  // writes a 500 with nothing in it. Calling res.json() blind would turn
  // those into a confusing parse error.
  const text = await res.text();
  const payload: unknown = text ? safeParse(text) : null;

  if (!res.ok) {
    if (res.status === 401 && !skipAuthRedirect) onUnauthorized?.();
    throw new ApiError(errorMessage(payload, res.status), {
      status: res.status,
    });
  }

  return payload as T;
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    // A 405 from ServeMux is plain text, not JSON.
    return { error: text.trim() };
  }
}

/**
 * Errors are always `{"error": "..."}` (api/internal/httpapi/errors.go), and
 * the message is a wrapped Go error chain. Callers branch on `status`, never
 * on this string.
 */
function errorMessage(payload: unknown, status: number): string {
  if (payload && typeof payload === "object" && "error" in payload) {
    const e = (payload as { error: unknown }).error;
    if (typeof e === "string" && e.trim() !== "") return e;
  }
  return `Request failed (${status})`;
}

function toNetworkError(err: unknown): ApiError {
  const offline =
    typeof navigator !== "undefined" && navigator.onLine === false;

  if (err instanceof DOMException && err.name === "TimeoutError") {
    return new ApiError("The server took too long to answer.", {
      isTimeout: true,
      isOffline: offline,
    });
  }
  if (err instanceof DOMException && err.name === "AbortError") {
    return new ApiError("Request cancelled.", { isOffline: offline });
  }
  return new ApiError(
    offline ? "You're offline." : "Could not reach the server.",
    { isOffline: offline },
  );
}

/* ------------------------------------------------------------------ auth */

export function login(email: string, password: string): Promise<User> {
  return apiFetch<User>("/api/auth/login", {
    method: "POST",
    body: { email, password },
    skipAuthRedirect: true,
  });
}

export function logout(): Promise<null> {
  return apiFetch<null>("/api/auth/logout", { method: "POST" });
}

export function me(
  opts: { skipAuthRedirect?: boolean; signal?: AbortSignal } = {},
): Promise<User> {
  return apiFetch<User>("/api/auth/me", opts);
}

/* ---------------------------------------------------------------- health */

export type Health = { status: string };

/** Unauthenticated (httpapi/router.go:12) — safe to call before login. */
export function health(signal?: AbortSignal): Promise<Health> {
  return apiFetch<Health>("/api/health", { signal, skipAuthRedirect: true });
}

/* ------------------------------------------------------------ categories */

/**
 * The API returns archived categories too (it filters only by user_id), so
 * they are dropped here rather than in every screen that lists them.
 */
export async function listCategories(): Promise<Category[]> {
  const all = await apiFetch<Category[]>("/api/categories");
  return all.filter((c) => !c.is_archived);
}

/* -------------------------------------------------------------- ledger */

export function listPeople(signal?: AbortSignal): Promise<Person[]> {
  return apiFetch<Person[]>("/api/people", { signal });
}

export function createPerson(input: PersonInput): Promise<Person> {
  return apiFetch<Person>("/api/people", { method: "POST", body: input });
}

export function deletePerson(id: string): Promise<null> {
  return apiFetch<null>(`/api/people/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function listDebtEntries(
  personID: string,
  signal?: AbortSignal,
): Promise<DebtEntry[]> {
  return apiFetch<DebtEntry[]>(
    `/api/people/${encodeURIComponent(personID)}/entries`,
    { signal },
  );
}

export function createDebtEntry(
  personID: string,
  input: DebtEntryInput,
): Promise<DebtEntry> {
  return apiFetch<DebtEntry>(
    `/api/people/${encodeURIComponent(personID)}/entries`,
    { method: "POST", body: input },
  );
}

export function settleDebtEntry(
  entryID: string,
  input: SettlementInput,
): Promise<Settlement> {
  return apiFetch<Settlement>(
    `/api/debt-entries/${encodeURIComponent(entryID)}/settle`,
    { method: "POST", body: input },
  );
}

export function settleAllDebtEntries(
  personID: string,
  input: SettlementInput,
): Promise<SettleAllResult> {
  return apiFetch<SettleAllResult>(
    `/api/people/${encodeURIComponent(personID)}/settle-all`,
    { method: "POST", body: input },
  );
}

export function deleteDebtEntry(id: string): Promise<null> {
  return apiFetch<null>(`/api/debt-entries/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/* -------------------------------------------------------------- budgets */

export function listBudgets(
  month: string,
  signal?: AbortSignal,
): Promise<BudgetWithSpent[]> {
  return apiFetch<BudgetWithSpent[]>(
    `/api/budgets?month=${encodeURIComponent(month)}`,
    { signal },
  );
}

export function upsertBudget(input: BudgetInput): Promise<Budget> {
  return apiFetch<Budget>("/api/budgets", { method: "PUT", body: input });
}

export function deleteBudget(id: string): Promise<null> {
  return apiFetch<null>(`/api/budgets/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/* ---------------------------------------------------------- transactions */

export function listTransactions(
  query: TransactionQuery = {},
  signal?: AbortSignal,
): Promise<TransactionList> {
  const params = new URLSearchParams();
  if (query.kind) params.set("kind", query.kind);
  if (query.month) params.set("month", query.month);
  if (query.category_id) params.set("category_id", query.category_id);
  if (query.q) params.set("q", query.q);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));

  const qs = params.toString();
  return apiFetch<TransactionList>(
    qs ? `/api/transactions?${qs}` : "/api/transactions",
    { signal },
  );
}

export function createTransaction(
  input: TransactionInput,
  idempotencyKey: string,
): Promise<Transaction> {
  return apiFetch<Transaction>("/api/transactions", {
    method: "POST",
    idempotencyKey,
    body: input,
  });
}

/** PATCH replaces the whole row — send every field. See TransactionInput. */
export function updateTransaction(
  id: string,
  input: TransactionInput,
): Promise<Transaction> {
  return apiFetch<Transaction>(`/api/transactions/${id}`, {
    method: "PATCH",
    body: input,
  });
}

export function deleteTransaction(id: string): Promise<null> {
  return apiFetch<null>(`/api/transactions/${id}`, { method: "DELETE" });
}

/* --------------------------------------------------------------- summary */

/** Current Karachi month dashboard. The API resolves the month at request time. */
export function getSummary(signal?: AbortSignal): Promise<Summary> {
  return apiFetch<Summary>("/api/summary", { signal });
}
