# Rakam

A private expense and lend/borrow ledger. Go API, Postgres, Next.js PWA frontend.

Single user. Currency is Pakistani Rupee (PKR). Timezone is Asia/Karachi.

Build in the phases at the end. Each phase must end with something that runs and is verified — no stubs carried forward.

---

## Stack

**API** — Go 1.23, standard library `net/http` (Go 1.22+ routing patterns), `pgx/v5` with `pgxpool`, `golang-migrate` for migrations, `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `log/slog`. Deployed to Render as a free web service via a multi-stage Dockerfile.

**Web** — Next.js 15 App Router, TypeScript strict, Tailwind v4. Deployed to Vercel.

**Database** — Neon Postgres.

Deliberately not used: no web framework, no ORM, no query generator, no component library, no state management library, no validation library. Everything here is small enough that these add more surface than they remove. Do not add dependencies beyond the list above without a stated reason.

### Repository layout

```
rakam/
  api/
    cmd/api/main.go          entrypoint, wiring, server start
    internal/
      domain/                entities, business rules, domain errors
      port/                  interfaces the services depend on
      service/               use cases
      postgres/              repository implementations, SQL
      httpapi/               handlers, middleware, router
      auth/                  password hashing, token issue and verify
      config/                env loading
      events/                in-process event bus
    migrations/
    Dockerfile
  web/
    app/                     routes
    components/
    lib/                     api client, money helpers, date helpers
    public/                  manifest, icons, service worker
  README.md
```

One repo, two deployables. Render builds from `api/`, Vercel builds from `web/`.

Everything runs on free tiers: Neon for Postgres, Render for the API, Vercel for the web app. Do not introduce a service that requires a paid plan or a credit card. If a task seems to need one, stop and ask.

---

## Architecture

Business logic must not know Postgres exists. Dependencies point inward: `httpapi` → `service` → `domain`, with `postgres` implementing interfaces that `port` defines.

Rules:

1. **`domain/` imports nothing outside the standard library.** No pgx, no net/http, no JSON tags on domain entities. If a domain file imports a driver, the layering is broken.
2. **Every repository is an interface in `port/`.** Implementations live in `postgres/` and are the only place SQL is written. Ports needed: `TransactionRepo`, `DebtRepo`, `PersonRepo`, `BudgetRepo`, `RecurringBillRepo`, `WorkLogRepo`, `CategoryRepo`, `UserRepo`.
3. **Services take dependencies as constructor parameters.** `NewTransactionService(txRepo port.TransactionRepo, bus port.EventBus) *TransactionService`. No package-level globals, no service importing a concrete repo.
4. **`main.go` is the only file that knows concrete types.** It opens the pool, constructs repositories, injects them into services, injects services into handlers, registers event subscribers, starts the server. Nothing else wires anything.
5. **Handlers are thin.** Decode JSON, validate, read user ID from request context, call one service method, map domain errors to status codes, encode the response. If a handler exceeds about 30 lines, logic has leaked out of the service.
6. **Repositories return domain types, not database rows.** Map at the boundary.
7. **Errors are domain sentinel values** — `domain.ErrNotFound`, `domain.ErrInvalidAmount`, `domain.ErrDuplicateBudget`. A single mapping function in `httpapi` turns them into status codes. Services never return raw pgx errors upward.
8. **Context flows everywhere.** Every repository and service method takes `ctx context.Context` first.

### Interfaces, kept honest

Define interfaces where the consumer lives, not next to the implementation, and keep them narrow — a service that only reads gets a read-only port. Do not write a generic `Repository[T]`. Do not add a decorator chain, a mediator, or a CQRS split. If a pattern isn't solving a problem named in this spec, leave it out. Over-layering is a defect, same as under-layering.

### Domain events

`port.EventBus` with an in-memory synchronous implementation in `events/`. Subscribers are registered in `main.go`. Exactly two subscribers in this build:

- `DebtSettled` → creates a transaction, only when the settle request asked for it.
- `TransactionCreated` → recomputes the affected budget and records whether a threshold was crossed.

Synchronous and in-process. No queue, no broker.

### Testing

Standard library `testing`, table-driven. Write in-memory fake repositories implementing each port and test services against them — no database needed for service tests.

Cover at minimum: net balance across mixed directions with partial settlement, budget threshold boundaries, recurring bill idempotency, Karachi month boundaries, and money parsing.

Repository tests are optional; if written, use a real Postgres and skip when `TEST_DATABASE_URL` is unset.

---

## Money

Stored and transported as **int64 paisa**. Never float, never a decimal string in business logic.

In `domain`, define `type Money int64` with methods `Rupees() float64` and `String() string` producing `Rs 12,852`. Parsing from user input lives in one place and handles both `"12852"` and `"12852.50"`.

The API sends and receives paisa as a JSON number. The web client formats for display and converts on input. Never do arithmetic on the formatted string.

---

## Data model

All tables: `id uuid primary key default gen_random_uuid()`, `user_id uuid not null references users(id) on delete cascade`, `created_at timestamptz not null default now()`, `updated_at timestamptz not null default now()`.

**`users`** — `email` unique, `password_hash`, `name`. Exactly one row, created by a seed command, never by a signup endpoint.

**`categories`** — `name`, `kind` (`expense` | `income`), `icon` (emoji), `color` (hex), `sort_order`, `is_archived` bool.

Seed expense categories: Food, Groceries, Transport, Rent, Bills & Utilities, Mobile & Internet, Shopping, Health, Travel, Entertainment, Family, Other. Income: Salary, Freelance, Other.

**`transactions`** — `kind` (`expense` | `income`), `amount_paisa` bigint, `category_id` uuid null, `description` text null, `occurred_at` timestamptz, `recurring_bill_id` uuid null.

Index `(user_id, occurred_at desc)`.

**`people`** — `name`, `phone` null, `notes` null. Seed: Usman, Moiz, Talha, Ali, Baba, SCB Rent.

**`debt_entries`** — `person_id` uuid, `direction` (`i_owe` | `they_owe`), `amount_paisa` bigint, `description` text, `incurred_at` timestamptz, `settled_at` timestamptz null.

A person's net balance is the sum of unsettled `they_owe` minus the sum of unsettled `i_owe`. Positive means they owe me. Settling stamps a timestamp — never delete, history must remain viewable.

A debt entry is not a transaction. Lending money and spending money are different events. The settle endpoint accepts an optional flag to also record a transaction; the user decides at settle time.

**`budgets`** — `category_id` uuid, `month` date (always the 1st), `limit_paisa` bigint. Unique `(user_id, category_id, month)`.

**`recurring_bills`** — `name`, `amount_paisa` bigint, `category_id` uuid, `day_of_month` int 1–31, `is_active` bool, `last_generated_month` date null.

**`work_logs`** — `month` date (always the 1st), `content` text (markdown). Unique `(user_id, month)`.

---

## API

All routes under `/api`. JSON in, JSON out. Every route except login requires a valid session.

```
POST   /api/auth/login              email, password → sets cookie
POST   /api/auth/logout
GET    /api/auth/me

GET    /api/categories
POST   /api/categories
PATCH  /api/categories/{id}
DELETE /api/categories/{id}          archives, does not hard delete

GET    /api/transactions             ?month=2026-08&category_id=&q=
POST   /api/transactions
PATCH  /api/transactions/{id}
DELETE /api/transactions/{id}

GET    /api/people                   includes net balance per person
POST   /api/people
GET    /api/people/{id}/entries
POST   /api/people/{id}/entries
POST   /api/debt-entries/{id}/settle    body: { create_transaction: bool }
POST   /api/people/{id}/settle-all
DELETE /api/debt-entries/{id}

GET    /api/budgets                  ?month=2026-08, includes spent per category
PUT    /api/budgets                  upsert by category + month
DELETE /api/budgets/{id}

GET    /api/bills
POST   /api/bills
PATCH  /api/bills/{id}
DELETE /api/bills/{id}

GET    /api/work-logs                list of months with a bullet count
GET    /api/work-logs/{month}        month as 2026-08
PUT    /api/work-logs/{month}        upsert content

GET    /api/summary                  ?month=2026-08, everything the home screen needs
GET    /api/export                   full JSON dump
```

Errors return `{ "error": "message" }` with a sensible status. Validation failures are 400 with a message naming the field.

### Middleware

Three, written by hand, composed in `main.go`: request logging with `slog`, panic recovery, and session authentication that puts the user ID into the request context. Nothing more.

### Recurring bill generation

A method on `RecurringBillService` that finds active bills whose `day_of_month` has passed in the current Karachi month and whose `last_generated_month` is not the current month, then creates the transaction and stamps the month — inside a transaction, so it is idempotent under concurrent calls. Clamp `day_of_month` to the last day of shorter months.

Call it from `GET /api/summary`. No scheduler, no cron.

---

## Auth

Single user, no signup route. A seed command (`go run ./cmd/api seed` or a small `cmd/seed`) creates the user from `SEED_EMAIL` and `SEED_PASSWORD`, hashing with bcrypt at default cost.

Login verifies the password and issues a JWT containing the user ID, 30-day expiry, set as an httpOnly, Secure, SameSite=Lax cookie. Middleware parses it and rejects with 401 when missing or invalid.

**Every repository query filters by user_id, taken from the session context — never from a request body or path.** This matters even with one user; it's the habit that keeps multi-tenant code correct later.

### Killing CORS

The Next.js app rewrites `/api/:path*` to the Render service URL in `next.config.ts`. The browser only ever talks to the Vercel origin, so the cookie is same-origin and no CORS configuration is needed anywhere.

Set `API_ORIGIN` as a Vercel environment variable and reference it in the rewrite. Do not add CORS middleware to the Go service; if it seems necessary, the rewrite is misconfigured.

---

## Frontend

Mobile-first. Fixed bottom tab bar: **Home · Expenses · Ledger · Budget · More**. A floating "+" button above the tab bar on every tab opens Add Transaction.

Mostly client components calling the API with `fetch` and `credentials: "include"`. One small typed API client in `lib/api.ts` — no data fetching library.

### The one quality bar that matters

Every expense tracker fails the same way: logging a purchase takes six taps and three dropdowns, so you stop logging. **Adding an expense must take under 5 seconds.**

- Opens straight to a large numeric keypad, amount focused.
- Category is a row of tappable chips, not a dropdown. Most-used first.
- Description optional. Date defaults to now, collapsed behind a "change" link.
- One button saves and closes.

If a feature slows that flow, it goes elsewhere in the app.

### Screens

**Home** — current month spent, earned, net. One overall budget progress bar with days remaining. A "money on the street" card showing total owed to me, total I owe, and the net, tapping through to Ledger. Bills due in the next 7 days. Last 5 transactions.

**Expenses** — reverse-chronological, grouped by day with a day subtotal on each header. Month picker, category filter, text search. Tap a row to edit or delete. Month total pinned at top.

**Ledger** — people list with net balance per row, green when they owe me, red when I owe them. Tap into a person for their history, unsettled first and settled below dimmed, with per-entry settle and a settle-all action. Add entries and add people from here.

**Budget** — current month, one row per category with a limit: name, spent over limit, progress bar. Amber past 80%, red past 100%. Categories without a limit listed below with their spend. Tap to set or edit inline. Month switcher for reviewing past months.

**Work log** — replaces the monthly notes I keep in Notion, which is too heavy for the job. A plain vertical list of months, newest first: "March 2026", "February 2026", "January 2026", each tappable, each showing how many bullets it holds. Months with no entry yet still appear for the current year, dimmed. Tapping opens a full-screen markdown editor for that month. It must feel like a notes app, not a form — no save button, autosave on a short debounce with a quiet "saved" indicator, Enter continues the bullet list, rendered markdown when unfocused. Include a copy-as-markdown action.

**More** — work log, recurring bills CRUD with next due date, categories management, JSON export, sign out.

### Visual direction

Do not ship default Tailwind — no white cards with `shadow-md` and a blue-500 button. Give it an identity.

Direction: a modern take on the handwritten ledger a shopkeeper keeps. Warm paper background rather than stark white, deep ink-green primary, muted gold for accents and warnings, brick red for money going out. Numbers in a tabular monospace so columns align and digits don't shift as values change; body text in a clean humanist sans. Money is the most important thing on every screen — set it large and aligned.

Dark mode reads as deep ink on warm charcoal, not the same palette inverted.

Quality floor, without being asked: responsive to 360px, visible focus rings, `prefers-reduced-motion` respected, 44px touch targets, safe-area insets so the tab bar clears the iPhone home indicator.

### PWA

`manifest.json` with name "Rakam", theme color, maskable icons at 192 and 512 (generate them). Service worker caching the app shell so it opens instantly and doesn't show a browser error offline. Set `apple-mobile-web-app-capable` and the status bar style.

Offline writes are out of scope. A mutation that fails offline shows a retry toast rather than silently losing data.

---

## Configuration

API env: `DATABASE_URL`, `JWT_SECRET`, `PORT`, `SEED_EMAIL`, `SEED_PASSWORD`.
Web env: `API_ORIGIN`.

Load config once at startup into a struct and fail fast with a clear message if anything required is missing. Provide `.env.example` for both. Include a `Makefile` with `run`, `test`, `migrate-up`, `migrate-down`, `seed`.

`README.md` covers local setup, running migrations, seeding, running both halves, and deploying to Railway plus Vercel plus Neon.

---

## Build phases

**1 — Foundation and one vertical slice.** Go module, config loading, pgxpool, migrations for the full schema, Dockerfile, `main.go` wiring, logging and recovery middleware. Build categories end to end as the pattern everything else copies: `domain.Category` → `port.CategoryRepo` → `postgres.CategoryRepo` → `service.CategoryService` → handlers → routes. Seed command for user and categories and people. *Verify:* migrations run clean, seed populates, `GET /api/categories` returns them, one service test passes against an in-memory fake.

**2 — Auth.** bcrypt hashing, JWT issue and verify, login, logout, me, session middleware, user-scoped queries. *Verify:* protected routes return 401 without a cookie and 200 with one, and the token survives a restart.

**3 — Transactions.** Domain, repo, service, handlers, filtering by month and category and search. Money type and parsing with tests. *Verify:* create, list, filter, update, delete all work over HTTP, and Karachi month boundaries are correct for a 2am purchase on the 1st.

**4 — Ledger.** People with net balances, entries, settle, settle-all. Event bus plus the `DebtSettled` subscriber. *Verify:* balances are correct across mixed directions with partial settlement, and settling with the flag creates a transaction while settling without it does not.

**5 — Budgets and bills.** Budgets with spent-per-category, recurring bills CRUD, idempotent generation, `TransactionCreated` subscriber, the summary endpoint. *Verify:* generation runs exactly once per bill per month under repeated calls.

**6 — Web foundation.** Next.js, Tailwind with the palette and type scale, the API rewrite, api client, login page, route protection, tab bar, and the add-transaction flow plus expenses list. *Verify:* the add flow genuinely takes under 5 seconds on a phone-sized viewport.

**7 — Remaining screens.** Ledger, budget, bills, categories, home dashboard, export. *Verify:* every endpoint from phase 3 to 5 has a working screen.

**8 — Work log, PWA, polish.** Work log API and editor, manifest, service worker, icons, dark mode, empty states, README. *Verify:* installs to a phone home screen, opens offline, and the work log persists without an explicit save.

---