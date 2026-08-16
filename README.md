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
make seed                 # create your user, categories and people
make run
```

`api/.env` needs all five of these or the service exits at startup with a
message naming what is missing:

| Variable | Notes |
|---|---|
| `DATABASE_URL` | Neon connection string |
| `JWT_SECRET` | at least 32 bytes — `openssl rand -base64 32` |
| `PORT` | e.g. `8091` |
| `SEED_EMAIL` | the one user's email |
| `SEED_PASSWORD` | the one user's password |

There is no signup route. `make seed` is the only way a user is created.

### 2. The web app

```sh
cd web
cp .env.example .env.local   # point API_ORIGIN at the API's port
npm install
npm run dev
```

Open http://localhost:3000 and sign in with `SEED_EMAIL` / `SEED_PASSWORD`.

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
cd api && make test && go vet ./...
cd web && npm run check          # tsc --noEmit, eslint, next build
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

- Every request has a 90 second timeout, so a cold start never fails.
- `GET /api/health` is called when the app opens. If it has not answered within
  2 seconds, a "waking up the server" screen appears with a spinner, and polls
  with backoff until the server answers.
- The server-side session check has a short 3 second budget. If it is blown,
  the app treats the session as *provisional*, not as logged out — it shows the
  waking screen and re-checks against the API before mounting anything that
  reads data. An unreachable API must never bounce you to the login page.

---

## Deploying

**Database — Neon.** Create a project, take the connection string, run
`make migrate-up` and `make seed` against it from your machine.

**API — Render.** New Web Service from `api/` using the Dockerfile. Set
`DATABASE_URL`, `JWT_SECRET`, `SEED_EMAIL`, `SEED_PASSWORD`. Render injects
`PORT` itself. Note the service URL.

**Web — Vercel.** New project with `web/` as the root directory. Set
`API_ORIGIN` to the Render service URL. Nothing else.

Set them up in that order — the web app throws at build time if `API_ORIGIN` is
missing, which is deliberate.

---

## Known gaps

- **A retried save can duplicate.** If a save times out after the server had
  already committed it, tapping Retry writes it twice. The API has no
  idempotency key. Retry is always manual, never automatic, so you are in the
  loop — but closing this properly needs an idempotency key on
  `POST /api/transactions`.
- **Work log and export do not exist yet** on either side. `WorkLogRepo` is in
  `SPEC.md` but was never built, and there is no `/api/export` handler.
- Offline writes are out of scope. A mutation that fails offline shows a retry
  toast rather than silently losing the data.
