# portofolio-api-go

A drop-in Go rewrite of the backend behind [hallojanu.me](https://hallojanu.me), originally
written in Express + Prisma. It serves the same JSON contract to the same frontend, reads and
writes the same PostgreSQL schema, and accepts the same credentials, so the two
implementations can be swapped without a migration.

- **Go 1.27, standard library first.** Routing uses `net/http`'s method and wildcard patterns.
  No web framework and no ORM. Dependencies: `pgx/v5`, `golang-jwt/jwt/v5` and `x/crypto/bcrypt`.
- **Proven compatible.** Unit tests verify a bcrypt hash, an API-token hash and a JWT produced
  by the Node libraries. `scripts/parity.sh` runs both APIs against one database and compares
  51 requests: reads, error paths, cross-issued credentials, and writes made by one API and
  read back through the other.
- **Tested.** 280+ tests, including table-driven validator tests, `httptest` handler tests with
  fake repositories, and integration tests against a real Postgres. The suite runs with `-race`.
- **Small and safe to run.** A distroless image (~37 MB uncompressed, two static binaries) runs as non-root. The server shuts down
  gracefully, logs with `log/slog`, and propagates request IDs.

## Architecture

```
cmd/api          entrypoint: config → pgx pool → router → graceful shutdown
cmd/admin        CLI: create-user / reset-password (replaces the default seeded admin)
internal/
  config         env loading + validation (refuses weak JWT secrets)
  db             pgxpool, transactions, error mapping, Prisma URL compatibility
  httpx          JSON I/O, typed field access for lenient validation, middleware
  auth           JWT, bcrypt, API tokens, and the three route guards
  authapi        /api/auth: login, Google sign-in, /me, per-IP rate limiting
  token          /api/admin/tokens: scoped API tokens, race-safe active limit
  project        /api/projects: validation, slug generation, transactional reorder
  testimonial    /api/testimonials
  blog           /api/blog
  orgexp         /api/org-experience
  sitecontent    /api/site-content: strict per-section validators
  upload         image/CV uploads (content sniffing, atomic writes) + static files
  server         wires every module behind global middleware
```

Each domain package follows the same shape: `validate.go` is pure, `repo.go` holds SQL behind
an interface, `handler.go` holds the HTTP layer, and `routes.go` has a single `Register` function.
Handlers depend on repository interfaces, so they are tested without a database.

### Access control

| Guard | Accepts | Used for |
|---|---|---|
| `JWTOnly` | admin JWT | `/api/auth/me`, token management, uploads |
| `AdminOrRead` | admin JWT, or any active `pf_` API token on GET/HEAD | `.../admin-list` |
| `AdminOrWrite` | admin JWT, or an active API token with scope `write` | every create/update/delete |

API tokens are stored only as SHA-256 hashes. The plaintext is shown once, at creation.

## Running locally

```bash
cp .env.example .env          # set JWT_SECRET (32+ chars)
make db-up                    # Postgres 16 on 127.0.0.1:55434 with the schema applied
set -a; . ./.env; set +a
go run ./cmd/admin create-user admin   # prompts for the password on stdin
make run                      # http://localhost:3001/api/health
```

| Command | What it does |
|---|---|
| `make test` | unit + handler tests |
| `make test-integration` | adds the Postgres-backed tests (uses `TEST_DATABASE_URL`) |
| `make cover` | coverage summary |
| `make build` | static binaries in `bin/` |
| `docker build -t portofolio-api-go .` | production image |
| `scripts/parity.sh <express-url> <go-url>` | contract comparison against the Express API |

Configuration (environment): `PORT`, `DATABASE_URL` (Prisma-style `?schema=public` is accepted),
`JWT_SECRET`, `CORS_ORIGINS`, `ADMIN_EMAILS`, `PUBLIC_DIR`, `LOG_FORMAT` (`text|json`),
`APP_ENV` (`development|production`), `TRUSTED_PROXIES` (reverse-proxy CIDRs whose `X-Forwarded-For` is trusted for per-client rate limiting).

The API is documented in [`docs/openapi.yaml`](docs/openapi.yaml), and the full product
requirements are in [`docs/PRD.md`](docs/PRD.md).

## Differences from the Express version

Status codes, response shapes and the Indonesian error messages are unchanged. The following
changes are deliberate:

| Area | Express | Go |
|---|---|---|
| JWT secret | silently falls back to a hard-coded default | server refuses to start without a 32+ char secret |
| CORS | reflects any origin, with credentials | explicit allowlist (`CORS_ORIGINS`) |
| Google sign-in | allowlist hard-coded; token sent in the query string | `ADMIN_EMAILS`; token in the `Authorization` header; 5 s timeout |
| Default admin | seed creates `admin / admin123` | no default user; `cmd/admin` creates one |
| Brute force | none | per-IP token bucket on login (5/min) → 429 with `Retry-After` |
| Login timing | returns early for unknown users | bcrypt always runs (no username enumeration) |
| Request size | unbounded JSON | 1 MB JSON limit → 413 |
| Uploads | trusts the client's MIME type | extension, MIME type and sniffed content must all match |
| Project reorder | silently accepts unknown ids | 400, and the transaction rolls back |
| Bad input types | many reach the DB and return 500 | rejected with a 400 and a clear message |
| Ordering ties | unspecified | stable, with `id` as the tiebreaker |
| API-token limit | count-then-insert race | enforced inside a locked transaction |

The Prisma project remains the owner of production migrations. `migrations/` holds a copy for
local development and CI.
