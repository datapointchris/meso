# meso — project guide

meso is a personal, mobile-first training app: a unified movement library that
compiles into workouts, cycles, sessions, stats, and a journal, drivable by the
`meso` CLI (no MCP). It follows the **nomad** product model wholesale — Go API +
Go CLI + Vue, Authelia edge auth, registry-pull deploy.

- **What it is, why, and the domain model:** [`README.md`](README.md) — the README is the spec. Read it before adding an entity or endpoint.
- **Reference build to copy patterns from:** `~/webapps/nomad/` (`CLAUDE.md`, `api/`, `cli/`, `.planning/cli-auth-design.md`).
- **Universal rules** (git, commits, Python/JS conventions, MCP, etc.) live in `~/.claude/CLAUDE.md` and are not restated here.
- **Current progress and settled decisions:** `.planning/status.md` (gitignored).

## Architecture — three subsystems

| Subsystem | Path   | Shape                                                                            |
| --------- | ------ | -------------------------------------------------------------------------------- |
| API       | `api/` | Go 1.26, `net/http` ServeMux, `pgx/v5`, goose migrations at startup, `slog` JSON |
| CLI       | `cli/` | Go/cobra thin REST client over the API — the agent + power-user surface          |
| Web       | `web/` | Vue 3 + TypeScript + Vite SPA, Authelia cookie SSO (no login page in-app)        |

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

Top-level commands are **training nouns only** — that is what lets `meso --help` read
as a description of the domain. Anything about the software rather than the training
goes under the `admin` namespace (`internal/cli/admin.go`), following HashiCorp's
`vault operator` / `consul operator` convention. `admin feedback` is its first
inhabitant; a new non-domain command belongs there, not at the root.

## Conventions specific to meso

Schema conventions (lookup tables not enums, `TEXT` columns, PK strategy, server-side filtering,
seed-carries-only-the-FK-backbone) and product independence are fleet standards — see
`~/dev/standards/data.md` and `~/dev/standards/api-design.md`. How they land here:

- **Lookup tables**: `movement_kinds`, `muscles`, `relationship_kinds`, `cycle_statuses`,
  `set_kinds`, `load_modes`.
  `movement_muscles.role ∈ primary|secondary` is a bounded sub-attribute of a join, so it uses a
  `CHECK` — a `CHECK` is not an enum type.
- **PKs**: catalog rows (`movements`, `workouts`) use `GENERATED ALWAYS AS IDENTITY` with a
  `UNIQUE` natural key on `.name` so the CLI import upserts idempotently. User-generated rows
  (`workout_sessions`, `fitness_log_entries`) will use UUIDv7. `workout_movements` is the
  ordered-join case: surrogate id plus `DEFERRABLE UNIQUE(parent_id, position)`, and
  `session_movements` / `session_sets` follow it.
- **Plan vs. performance**: `session_movements.target_*` is the prescription copied from the
  workout; `session_sets` is what was actually done. They are never the same columns again —
  merging them is what made a diverged session read as a failed one.
- **Seed**: `movement_kinds`, `relationship_kinds`, `cycle_statuses`, `set_kinds`, `load_modes`,
  `muscles` only. Everything
  else — metrics, movements, workouts, cycles, measurements, journal — loads through the `meso`
  CLI.
- **Product independence**: nothing here may know about ichrisbirch, icb, or any sibling app. No
  foreign ids in the schema, no cross-service credentials, no outbound pushes. `feedback` is the
  live example — stored and triaged here, never forwarded. Don't re-propose a push integration.
- **No MCP, ever** — the `meso` CLI is the sole programmatic/agent surface.

Web conventions: every dialog goes in `components/ModalShell.vue`, every "are you sure" through
`composables/useConfirm.ts` — never `window.confirm`. `npm run typecheck` must keep its `--build`
flag; without it the root tsconfig is solution-style and nothing is checked.

## Local development

```bash
# Full stack (Postgres + API + Caddy SPA):
docker compose -f docker-compose.dev.yml up --build
docker compose -f docker-compose.dev.yml run --rm --entrypoint ./meso-seed api  # seed once

# Or run pieces directly:
cd api && DATABASE_URL=postgres://meso:meso@localhost:5459/meso?sslmode=disable go run .   # applies migrations, serves :8088
cd api && go run ./cmd/seed        # FK-backbone lookups + muscles (idempotent)
cd web && npm run dev              # Vite :3001, proxies /api → :8088
cd cli && go run . movements list  # needs `meso auth login` against Authelia
```

Ports: API **8088**, Postgres **5459**, Vite **3001**, Caddy web **8080**.

**Gotcha — local API tests need Docker reachable by testcontainers.** `go test ./...`
in `api/` spins up a throwaway Postgres via testcontainers. If auto-detection fails
(e.g. Docker Desktop / OrbStack / colima with a non-default socket), export
`DOCKER_HOST` at the socket, or set `TEST_DATABASE_URL` to reuse an existing DB. CI's
default `/var/run/docker.sock` needs nothing. This is a machine concern, not a repo change.

## Gates (all must pass before commit)

`task lint` and `task test` run these across all three subsystems; the per-subsystem
verbs are `task {api,cli,web}:{lint,test}`.

- **api / cli:** `gofmt -l .` (clean), `go vet ./...`, `go test ./...`
- **web:** `npm run lint`, `npm run typecheck`, `npm run test`, `npm run build`, `npm run format:check`

`task cli:install` puts `meso` on your PATH at `$GOBIN` (falling back to `$GOPATH/bin`)
with a git-derived version embedded via ldflags. Released binaries come from CI on a
`cli/v*` tag, not from a developer machine.
