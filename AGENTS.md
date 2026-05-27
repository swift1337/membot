# AGENTS.md

## Project Shape

`membot` is a small Go CLI that indexes local assistant history into SQLite and
queries it from the terminal. Keep changes simple, explicit, and close to the
existing package boundaries.

## Where To Look

- `cmd/membot/`: Cobra CLI commands, flags, JSON/text output wiring.
- `internal/store/`: SQLite opening, pragmas, embedded migrations, DB lifecycle.
- `internal/store/migrations/`: schema migrations consumed at startup.
- `internal/db/queries/`: sqlc query definitions; edit these before generated Go.
- `internal/db/`: sqlc-generated code plus small hand-written repository helpers.
- `internal/indexer/cursor/`: Cursor transcript discovery, parsing, and indexing.
- `internal/query/`: search, FTS query building, result shaping, text rendering.

## Workflow

- Run `make lint` for every feature before handing it off.
- Run `make codegen` after changing migrations or `internal/db/queries/*.sql`.
- Run focused `go test ./...` or package tests when behavior changes.
- Do not hand-edit sqlc-generated files; regenerate them.

## Dependencies

- Go is the implementation language; prefer standard library code first.
- Cobra (`github.com/spf13/cobra`) owns CLI structure.
- SQLite uses `modernc.org/sqlite` through `database/sql`.
- sqlc generates typed DB access from `sqlc.yaml`, migrations, and query files.
- golangci-lint runs via `go tool golangci-lint` in `make lint`.

## Style

- Prefer straightforward functions over new abstractions.
- Keep errors contextual with `fmt.Errorf("context: %w", err)`.
- Preserve JSON output contracts unless the user asks to change them.
- Add tests near the package being changed; keep fixtures minimal.
