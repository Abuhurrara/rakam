# Working rules for this repo

Read `SPEC.md` for what to build. This file is how to work here.

## Ground rules

- Go 1.23. Standard library `net/http` routing — do not add chi, gin, echo, or any router.
- Do not add a Go module or npm package that isn't already in `SPEC.md`. If you think one is needed, stop and ask first with a one-line reason.
- Hand-written SQL with pgx. No ORM, no sqlc, no query builder.
- `internal/domain` imports only the standard library. If you need pgx or net/http in there, the design is wrong — fix the design, not the import.
- Every repository is an interface in `internal/port`, implemented in `internal/postgres`. Services take dependencies as constructor parameters. Only `cmd/api/main.go` knows concrete types.
- Money is `int64` paisa everywhere. Never float. Never arithmetic on a formatted string.
- Every query filters by `user_id` read from request context — never from a body or path parameter.
- Tests are table-driven, standard library `testing`. Service tests run against in-memory fakes, not a database.

## Before you say a phase is done

- `make test` passes.
- `go vet ./...` is clean.
- The phase's own "Verify" line in `SPEC.md` is actually checked, not assumed.
- Tell me plainly what you did not finish or had to work around. Do not report success on partial work.

## Migrations

Never edit a migration file that has already been applied. Write a new one.

## Style

- Wrap errors with `fmt.Errorf("doing thing: %w", err)`. Return domain sentinel errors from services, never raw pgx errors.
- `context.Context` is the first parameter of every repository and service method.
- Keep handlers thin. If one passes ~30 lines, logic belongs in a service.