# meso — project guide

meso is a personal, mobile-first training app: a unified movement library that
compiles into workouts, cycles, sessions, stats, and a journal, drivable by the
`meso` CLI (no MCP). It follows the **nomad** product model wholesale — Go API +
Go CLI + Vue, Authelia edge auth, registry-pull deploy.

- **What it is, why, and the phase plan:** [`docs/design.md`](docs/design.md) — the design doc is the spec. Read it before adding an entity or endpoint.
- **Reference build to copy patterns from:** `~/webapps/nomad/` (`CLAUDE.md`, `api/`, `cli/`, `.planning/cli-auth-design.md`).
- **Universal rules** (git, commits, Python/JS conventions, MCP, etc.) live in `~/.claude/CLAUDE.md` and are not restated here.
- **Current progress and settled decisions:** `.planning/status.md` (gitignored).

## Architecture — three subsystems

| Subsystem | Path   | Shape                                                                            |
| --------- | ------ | -------------------------------------------------------------------------------- |
| API       | `api/` | Go 1.26, `net/http` ServeMux, `pgx/v5`, goose migrations at startup, `slog` JSON |
| CLI       | `cli/` | Go/cobra thin REST client over the API — the agent + power-user surface          |
| Web       | `src/` | Vue 3 + TypeScript + Vite SPA, Authelia cookie SSO (no login page in-app)        |

The CLI's natural sibling is the API (shared REST contract), not the Vue frontend
— that is why the CLI is Go, not TS.

### API layering (one file per resource)

Mirrors nomad: a resource flows through `models/` (wire structs + Create/Update/
Filter shapes) → `repository/` (all pgx SQL) → `handlers/` (HTTP, maps repo errors
to status codes) → routes in `main.go`. `migrations/` holds goose DDL only; data
lives in `cmd/seed`. Handler tests are testcontainers-backed (`handlers/main_test.go`).

### CLI layering

`internal/config` (OIDC/API settings) → `internal/auth` (PKCE flow + OS-keychain
token store) → `internal/api` (typed REST client over the wire contract) →
`internal/cli` (cobra command tree). Resource commands live one file per resource
under `internal/cli`, each with a matching `internal/api/<resource>.go` client.

## Conventions specific to meso

- **Lookup tables, never enum types** — categoricals (`movement_kinds`, `muscles`, `relationship_kinds`, `cycle_statuses`) are `TEXT PRIMARY KEY` + FK. A _bounded sub-attribute_ of a join (e.g. `movement_muscles.role ∈ primary|secondary`) uses a `CHECK`, not a 2-row lookup — a CHECK is not an enum type.
- **PK strategy** — catalog rows (`movements`) use Postgres `GENERATED ALWAYS AS IDENTITY`; user-generated rows (`workout_sessions`, `fitness_log_entries`) will use UUID7. Catalog rows carry a natural key (`movements.name UNIQUE`) so seed/import can upsert idempotently.
- **All text columns are `TEXT`** (never `varchar(n)`); array columns are `NOT NULL DEFAULT '{}'` and normalized nil→`[]` on write so reads are branch-free.
- **Filtering is server-side** — list endpoints own their query params and build the `WHERE` in SQL, so the CLI and web share one filter definition rather than each re-implementing it.
- **Seed writes through the repository**, not raw SQL, so `cmd/seed` exercises the real write path (transactions, FK validation). Lookups seed via direct upsert; catalog rows via `Repo.Upsert`.
- **No MCP, ever** — the `meso` CLI is the sole programmatic/agent surface.

## Local development

```bash
# Full stack (Postgres + API + Caddy SPA):
docker compose -f docker-compose.dev.yml up --build
docker compose -f docker-compose.dev.yml run --rm --entrypoint ./meso-seed api  # seed once

# Or run pieces directly:
cd api && DATABASE_URL=postgres://meso:meso@localhost:5459/meso?sslmode=disable go run .   # applies migrations, serves :8088
cd api && go run ./cmd/seed        # muscles + baseline movement catalog
npm run dev                        # Vite :3001, proxies /api → :8088
cd cli && go run . movements list  # needs `meso auth login` against Authelia
```

Ports: API **8088**, Postgres **5459**, Vite **3001**, Caddy web **8080**.

**Gotcha — local API tests need Docker reachable by testcontainers.** `go test ./...`
in `api/` spins up a throwaway Postgres via testcontainers. If auto-detection fails
(e.g. Docker Desktop / OrbStack / colima with a non-default socket), export
`DOCKER_HOST` at the socket, or set `TEST_DATABASE_URL` to reuse an existing DB. CI's
default `/var/run/docker.sock` needs nothing. This is a machine concern, not a repo change.

## Gates (all must pass before commit)

- **api / cli:** `gofmt -l .` (clean), `go vet ./...`, `go test ./...`
- **web:** `npm run lint`, `npm run typecheck`, `npm run test`, `npm run build`, `npm run format:check`
