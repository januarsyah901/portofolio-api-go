# AGENTS.md — conventions for anyone (human or agent) working in this repo

Read `docs/PRD.md` first. The Express reference implementation lives at
`../portofolio-api/src` (read-only, never edit it). Behaviour, status codes and
Indonesian error messages must match it exactly unless PRD §5 lists a deliberate change.

## Toolchain
- Go is installed at `~/.local/go/bin` — run `export PATH=$HOME/.local/go/bin:$PATH` first.
- `make test` (unit + handler), `make vet`, `make test-integration` (needs `make db-up`;
  dev Postgres on 127.0.0.1:55434, credentials in `.env.example`).
- `gofmt -s -w .` before committing. `go vet ./...` must be clean.

## Module layout (one package per domain under `internal/`)
| File | Content |
|---|---|
| `routes.go` | `func Register(mux *http.ServeMux, d app.Deps)` — already stubbed; fill it in. Build the real repo from `d.Pool` here. |
| `validate.go` | Pure validation, no I/O. Port of the matching `src/lib/*.ts`. |
| `repo.go` | SQL via pgx. Define a `Repo` interface in the package; a `pgRepo` struct implements it. |
| `handler.go` | `Handler` struct holding the `Repo` interface; one method per endpoint. |
| `*_test.go` | Table-driven unit tests for validate.go; handler tests with `httptest` and an in-memory fake Repo. |
| `*_integration_test.go` | `//go:build integration`; use `testutil.DB(t)`; real CRUD round trips. |

## Rules
- Routing: stdlib `net/http` patterns, e.g. `mux.Handle("PUT /api/projects/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.update)))`.
  Use `d.Guards.JWTOnly / AdminOrRead / AdminOrWrite` exactly as PRD §4.1 assigns them.
- Request bodies: `fields, ok := httpx.DecodeObject(w, r); if !ok { return }` then use
  `httpx.Fields` helpers (`String`, `Bool`, `Number`, `Int`, `StringSlice`, `IsNull`, `Has`, `Truthy`)
  to reproduce the JS `typeof` checks. Do not decode straight into structs for input.
- Path ids: `id, ok := httpx.PathInt(r, "id")`; on !ok return 400 with the module's Express message.
- Responses: `httpx.JSON(w, status, v)`, `httpx.Error(w, status, msg)`, and
  `httpx.ServerError(w, r, "label", err)` for 500s (message "Terjadi kesalahan pada server.").
- JSON models: struct tags in camelCase matching Prisma field names; **no `omitempty`**;
  nullable columns are pointer types (`*string`, `*time.Time`) so they serialise as `null`;
  arrays must serialise as `[]`, never `null` — use `COALESCE("tags", '{}')` in SELECTs.
- SQL: always double-quote identifiers (`"Project"."createdAt"`). Set `"updatedAt" = now()`
  yourself on insert/update where the column exists (Prisma's @updatedAt is client-side).
  Map `pgx.ErrNoRows` with `db.NotFound(err)`; detect unique violations with `db.IsUniqueViolation`.
- Use `r.Context()` for every DB call. No global state, no `init()` side effects.
- Stay inside your own packages. Do not edit `internal/{app,auth,config,db,httpx,server,testutil}`,
  `go.mod` dependencies, or other modules. If you truly need a shared change, describe it in
  your final report instead of making it.
- Commit with conventional messages (`feat(project): ...`, `test(blog): ...`).
