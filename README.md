# Rakam

A private expense and lend/borrow ledger. Go API, Postgres, Next.js web app.

Single user. Currency is Pakistani Rupee (PKR). Timezone is Asia/Karachi.

See `SPEC.md` for what is being built and `CLAUDE.md` for how to work in this repo.

```
rakam/
  api/     Go service — deployed to Render
  web/     Next.js app — deployed to Vercel
```

One repo, two deployables.

## Live release — September 9, 2026

- App: [rakam-five.vercel.app](https://rakam-five.vercel.app/)
- API: [raqam-nrro.onrender.com/api/health](https://raqam-nrro.onrender.com/api/health)
- Release commit: `1f86a93`; frontend and backend deployed successfully.
- Database migrations through `0003_transaction_requests` are applied.

This release fixes duplicated keypad input, adds recoverable expense saves and
JSON export, corrects filtered totals and edit timestamps, and removes backend
authentication from app-route server rendering.

Live checks passed for all five tabs, entering and saving exactly **1200**, retrying
a save without duplication, JSON download, API health, and PWA assets. Temporary
verification entries were removed. Phone-sized browser checks passed with no
captured browser errors; the installed app on a physical Android phone still
needs verification. At that release, Home, Ledger and Budget were placeholder
screens.

Since then, Home, Expenses and Ledger have been implemented and deployed.
Ledger keeps an account-scoped local snapshot so returning to its list or a
person's history renders immediately, then refreshes stale data quietly in the
background. The Budget screen was subsequently deployed. It shows the current
Karachi month, supports past-month limits, and keeps account-scoped month
snapshots visible while refreshing. This branch changes over-limit display to
show exact rupees over and counts only limited categories in budget progress;
that change is not yet deployed. The management areas in More are still
placeholders.

To load the update, close and reopen or reload the installed PWA. Do not clear
site storage: that would remove any unfinished local drafts.

Neon already runs PostgreSQL. At this release, Render is in Oregon and Neon is
in Frankfurt. Their distance and free-tier sleep can still delay data requests;
the navigation fix does not remove those hosting delays. Hosting is unchanged.
Invite-only accounts remain deferred.

---

## What you need

- Go 1.26
- Node 20+
- A Postgres database (Neon free tier is what this is built for)
- `golang-migrate` CLI, for running migrations

---

## Running it locally

### 1. The API

```sh
cd api
cp .env.example .env      # then fill it in
make migrate-up           # create the schema
make user-create EMAIL=you@example.com NAME="Your Name"
make run
```

`api/.env` needs these three runtime values:

| Variable | Notes |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string (local or Neon) |
| `JWT_SECRET` | at least 32 bytes — `openssl rand -base64 32` |
| `PORT` | e.g. `8091` |

For legacy local sample data only, `make seed` also needs `SEED_EMAIL` and
`SEED_PASSWORD` in `api/.env`; it creates example categories and people.

There is no public signup route. To create a private, empty account for a friend, apply migrations and run:

```sh
cd api
make migrate-up
make user-create EMAIL=friend@example.com NAME="Friend Name"
```

The command prompts for a password twice without echoing it. Passwords must be
12–72 bytes. It only creates the account; it does not create categories,
people, budgets, or finance data. New users can record uncategorized expenses
until category management is available. To reset an account password:

```sh
make user-reset-password EMAIL=friend@example.com
```

A reset signs out all devices. Users can optionally change their own password
from More; that also signs out their other devices. `make seed` is retained
for local development fixtures only and should not be used for production
account creation.

### 2. The web app

```sh
cd web
cp .env.example .env.local   # point API_ORIGIN at the API's port
npm install
npm run dev
```

Open http://localhost:3000 and sign in with the account credentials created above.

**`API_ORIGIN` must match the `PORT` you set in `api/.env`.** It is the only
variable the web app needs.

---

## How the two halves talk

The browser never calls the Go service directly. `web/next.config.ts` rewrites
`/api/:path*` to `API_ORIGIN`, so every request goes to the Next.js origin and
is proxied server-side.

That is what makes the session cookie same-origin, and it is why there is no
CORS configuration anywhere in this project. If CORS ever seems necessary, the
rewrite is misconfigured — do not add CORS middleware to the Go service.

`API_ORIGIN` is deliberately **not** a `NEXT_PUBLIC_` variable. It is read only
on the server, in `next.config.ts` and `web/lib/server-api.ts`, so it never
reaches the browser bundle.

### Local gotcha: use `localhost`, not a LAN IP

The session cookie is set with `Secure: true` and no dev toggle. Browsers make
an exception for `localhost`, so `http://localhost:3000` works fine. They do
**not** make that exception for `http://192.168.1.x:3000` — the cookie is
silently dropped and signing in appears to do nothing.

To test on a real phone, put it behind HTTPS (a tunnel such as `ngrok` or
`cloudflared`), or deploy it.

---

## Checks

```sh
(cd api && make test && go vet ./...)
(cd web && npm run check)       # regression tests, TypeScript, ESLint, production build
```

---

## Money

Money is `int64` paisa everywhere — never a float, never arithmetic on a
formatted string.

The wire contract is asymmetric on purpose, and it is easy to get wrong:

- **Sending**: `amount` is the raw **string** the user typed, e.g. `"1250.50"`.
  The Go side parses it with `domain.ParseMoney`. The web app never parses a
  money value in JavaScript.
- **Receiving**: every money field is `*_paisa`, an **integer**, e.g. `125050`.
  The web app formats it for display with integer arithmetic only.

`web/lib/money.ts` is the only place money is formatted or validated:

- `formatPaisa(125050)` → `"Rs 1,250.50"`. Paisa are shown when the value is
  not a whole number of rupees, and hidden when it is, so a column of rows
  always visibly sums to the total printed above it.
- `isValidAmountInput(raw)` mirrors the server's `^\d+(\.\d{1,2})?$` exactly,
  so the client refuses precisely what the server would refuse.

## Dates

Everything is Asia/Karachi, never the device's timezone. `web/lib/date.ts`
converts through `Intl` with an explicit `timeZone`.

`occurred_at` goes over the wire as RFC3339 with the Karachi offset
(`2026-08-16T14:30:00+05:00`). A bare date string is rejected with a 400.

When grouping the list by day, always convert the timestamp the API returned
through `karachiDayKey`. Do not slice the first ten characters of it — Go
formats it from whatever offset the database session is in, so slicing files a
2am Karachi purchase under the previous day.

---

## Cold starts

The API sleeps on Render's free tier and can take most of a minute to answer
its first request. The web app handles this rather than surfacing it as an
error:

- Requests allow up to 90 seconds for a sleeping API to respond.
- App routes render a static, non-personal shell without a backend request.
  Middleware redirects missing cookies; AppShell verifies the session in the
  browser before mounting private screens. API authorization remains mandatory.
- The session request also wakes the server, removing the health-then-session
  waterfall. After 2 seconds the shell explains the wait; failures offer retry.
- The shared shell retains the verified session across tab changes. Tab routes
  are prefetched and have a loading boundary; expense data loads separately.
- Home and Expenses keep a versioned, account-scoped snapshot after the first
  successful load. Returning to either screen renders that snapshot immediately;
  data older than 30 seconds refreshes without replacing it with a skeleton.
  A server-confirmed create, edit or delete updates the cached Home totals
  immediately, then invalidates the snapshot so the full dashboard still
  reconciles with PostgreSQL in the background. Signing out clears the
  snapshot.
- Only the login page retains the server-side session check (3 second budget).
  Its form uses a health banner while Render wakes.
- No authenticated HTML or financial API responses are cached by the service
  worker. The verified-session snapshot is managed by the app rather than the
  worker, so it cannot bypass the authentication gate.

---

## Deploying

**Database — Neon.** Create a project, take the connection string, run
`make migrate-up` against it from your machine, then create the owner account
with `make user-create EMAIL=... NAME=...`. `make seed` is only for local
development fixtures and inserts example categories and people.

**API — Render.** New Web Service from `api/` using the Dockerfile. Set
`DATABASE_URL`, `JWT_SECRET`. Render injects
`PORT` itself. Note the service URL.

**Web — Vercel.** New project with `web/` as the root directory. Set
`API_ORIGIN` to the Render service URL. Nothing else.

Set them up in that order — the web app throws at build time if `API_ORIGIN` is
missing, which is deliberate.

---

## Known gaps

- **Work log does not exist yet** on either side. `WorkLogRepo` is in
  `SPEC.md` but was never built. Recurring-bill/category management screens
  remain unfinished. The Budget overage change still needs deployment.
- Full offline editing and automatic background sync are not implemented.
  Failed create/update requests retain a local draft for manual retry after
  reconnecting. An offline page load shows the offline fallback.
- Public signup and email-based password recovery are not implemented. The owner provisions accounts with the CLI; users may change their own password from More.


## Reliable saves and expense totals

The keypad uses one click activation for touch, mouse and keyboard, avoiding
pointer/click double handling that could duplicate digits on Android.

The web app sends an `Idempotency-Key` for each new expense. Retries
reuse the same key and payload. Migration `0003_transaction_requests` stores a
receipt atomically with the expense, so simultaneous retries create one entry.
Reusing a key with a different payload returns 409. Receipts survive deletion
of the expense, so a late retry cannot recreate a deleted record. Older clients
without a key still work but do not receive duplicate-save protection.

Before making a create/update request, the app persists an account-scoped draft
on the device. A timeout, reload or expired session keeps it available after
signing back into the same account. Recovered drafts require manual Retry; they
are never silently replayed. A successful response removes the draft. Discard
removes only the local draft, not an expense that may already have reached the
server. Clearing browser/site storage removes these drafts. If storage is
unavailable, the entry sheet stays open and no request is started.

Expense lists support `kind=expense`, pages of 50 rows in the UI, and an
`expense_paisa` total across **all** matching records. The server reads rows,
count and total in one consistent snapshot. Mutations reload the active query,
so changing a date, note or category also updates filter membership correctly.
Editing without a date change preserves the original timestamp.

For future releases, apply any pending migrations to the production database,
deploy the API, then deploy the web app. Migration 0003 is already applied to
this deployment; do not rerun its SQL manually or reseed existing accounts. Reopen/reload an existing installed PWA to load the updated keypad. The
worker does not cache authenticated HTML; no site-data clearing is required.

## Export and recovery

More → Download JSON backup downloads every saved transaction, category
(including archived), person, debt entry (including settled), budget, recurring
bill and work log for the signed-in user. `/api/export` returns a versioned JSON
snapshot with PKR paisa integers preserved exactly. Credentials and internal
retry receipts are excluded. Unsaved device drafts are not included.

The JSON file is a portable data export; a one-click JSON import is not provided.
Optional: for full database disaster recovery, use PostgreSQL's dump/restore tools
with a custom-format dump, which also preserves schema and retry receipts:

```sh
pg_dump --format=custom --file=rakam-backup.dump "$DATABASE_URL"
pg_restore --exit-on-error --no-owner --no-privileges \
  --dbname="$RESTORE_DATABASE_URL" rakam-backup.dump
```

Use a `pg_dump` version at least as new as the database server's major version
(the production server was PostgreSQL 18 at this release).

`RESTORE_DATABASE_URL` must point to a separate, empty database. Verify counts,
sign-in and several known expenses there before switching the API to it. Store
backups privately outside the repository; database dumps contain password hashes.
Run restore drills periodically rather than treating a download as proof of
recoverability.

## Regression checks

`cd web && npm run test` exercises keypad activation (including long holds),
Karachi timestamp preservation, persistent draft recovery, and rendering each
app shell without backend requests or mounting private client screens. These
checks use the existing dependencies. `npm run check` includes these tests.

Set `TEST_DATABASE_URL` to a disposable migrated PostgreSQL database when running
`make test`. Database tests verify concurrent save retries, key conflicts,
user isolation, deletion followed by retry, paginated totals, and export precision.
Never point tests at a production database.
