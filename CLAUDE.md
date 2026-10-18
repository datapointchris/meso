# meso — project guide

meso is a personal, mobile-first training app: a unified movement library that
compiles into workouts, cycles, sessions, stats, and a journal, drivable by the
`meso` CLI (no MCP). It is a Go API, a Go CLI and a Vue SPA, behind Authelia edge
auth, deployed by pulling images from a registry.

- **What it is, why, and the domain model:** [`README.md`](README.md) — the README is the spec. Read it before adding an entity or endpoint.

## Architecture — three subsystems

| Subsystem | Path   | Shape                                                                            |
| --------- | ------ | -------------------------------------------------------------------------------- |
| API       | `api/` | Go 1.26, `net/http` ServeMux, `pgx/v5`, goose migrations at startup, `slog` JSON |
| CLI       | `cli/` | Go/cobra thin REST client over the API — the agent + power-user surface          |
| Web       | `web/` | Vue 3 + TypeScript + Vite SPA, Authelia cookie SSO (no login page in-app)        |

The CLI's natural sibling is the API (shared REST contract), not the Vue frontend
— that is why the CLI is Go, not TS.

### API layering (one file per resource)

A resource flows through `models/` (wire structs + Create/Update/Filter shapes) →
`repository/` (all pgx SQL) → `handlers/` (HTTP, maps repo errors to status codes) →
routes in `handlers/router.go`. `migrations/` holds goose DDL only; data
lives in `cmd/seed`. Handler tests are testcontainers-backed (`handlers/main_test.go`).

### CLI layering

`internal/config` (where the CLI is pointed, read from `~/.config/meso/config.toml` through
goclilogin's `LoadDeployment`, and `Login()` mapping that onto goclilogin) →
`internal/api` (typed REST client over the wire contract) → `internal/cli` (cobra command
tree). Resource commands live one file per resource under `internal/cli`, each with a
matching `internal/api/<resource>.go` client.

Top-level commands are **training nouns only**, so `--help` reads as the domain rather than
as plumbing, and anything about the software rather than the training goes under `admin`
(`internal/cli/admin.go`). Vault and Consul call that namespace `operator`; it is `admin`
here because `operator` there means cluster lifecycle. The server refuses `admin` routes to
anyone outside the admin group, so the CLI only carries the door. `admin feedback` is its
first inhabitant; a new non-domain command belongs there, not at the root. `update` is the
one exception, because the update notice can only name a root command.

**A command that cannot answer offers the commands that can.** An error owes what failed,
what was expected, and the one command that changes the situation, and `goclikit.Execute`,
the shared cobra bootstrap, adds the near matches and the `--help` pointer cobra leaves
out. What is meso's own is the step past it: where the caller typed something valid and the CLI
still has nothing to show, the response is a list of runnable commands rather than a
failure at all. `resolveIDOrName` in `internal/cli/helpers.go` is the pattern. It writes
the menu itself and returns `exitCode`, which sets the process status without `Execute`
printing an `error:` line above it, and under `--json` it emits the same candidates as
data. Every command it composes takes its path from `cmd.CommandPath()` and its values
through `shellArg`, so a menu line can be pasted back as typed.

**It reaches `show` on movements, workouts and cycles, and nothing else yet.** Every other
verb returns plain errors through `handleAPIError`, and widening it belongs
in `goclikit` rather than here — so treat this as describing three commands, not the CLI.

**A name is accepted where a command reads and never where it writes.** `show` resolves
`<id-or-name>`; `edit`, `delete` and the composition verbs take ids. A fuzzy match that
narrows to the wrong record costs a wrong screen on a read and a wrong row on a write.

## Conventions specific to meso

The schema uses lookup tables rather than enums, `TEXT` columns, and server-side filtering, and
its seed carries only the FK backbone. How that lands here:

- **Lookup tables**: `movement_kinds`, `muscles`, `relationship_kinds`, `cycle_statuses`,
  `set_kinds`, `load_modes`, and those are exactly what `seed` carries. Everything else
  (metrics, movements, workouts, cycles, measurements, journal) loads through the `meso` CLI.
- **PKs**: catalog rows (movements, workouts, cycles) have an identity PK and a `UNIQUE` name as
  their natural key, which is what lets `show <name>` resolve to one row. `workout_sessions`,
  `fitness_log_entries` and `feedback` are on UUID, and `session_movements` / `session_sets` are
  ordered-join rows.
- **Plan vs. performance**: `session_movements.target_*` is the prescription copied from the
  workout; `session_sets` is what was actually done. They are never the same columns again —
  merging them is what made a diverged session read as a failed one.
- **Product independence**: the dependency points inward. `feedback` is stored and triaged here
  and forwarded nowhere; whoever wants it reads it out with `--json`. Storing another system's
  primary key would make this schema unreadable without theirs, so integration goes through
  stable data interfaces (JSON over HTTP, or read-only views), never shared mutable state. Don't
  re-propose a push integration.

Web conventions: every dialog goes in `components/ModalShell.vue`, every "are you sure" through
`composables/useConfirm.ts` — never `window.confirm`. `npm run typecheck` must keep its `--build`
flag; without it the root tsconfig is solution-style and nothing is checked.

## Local development

```bash
docker compose -f docker-compose.dev.yml up --build                               # Postgres + API + Caddy SPA
docker compose -f docker-compose.dev.yml run --rm --entrypoint ./meso-seed api    # seed once
cd api && DATABASE_URL=postgres://meso:meso@localhost:5459/meso?sslmode=disable PUBLIC_URL=http://localhost:8088 go run .   # migrates, serves :8088
cd web && npm run dev                                                             # Vite :3001, proxies /api → :8088
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

`task cli:install` writes `meso` to `~/.local/bin`, replacing the installed release, with a
git-derived version embedded via ldflags. A locally built one reports as a dev build and
refuses to self-update, by design rather than as a defect: `git describe` returns the prefixed
tag (`cli/v0.2.0-3-gabc1234`), which is not a semantic version. The CLI is a module in a
subdirectory, so it needs its full module path and a `cli/v*` tag before `go install` can
resolve it. Released binaries come from CI on a `cli/v*` tag.
