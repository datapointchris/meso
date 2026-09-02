# meso — project guide

meso is a personal, mobile-first training app: a unified movement library that
compiles into workouts, cycles, sessions, stats, and a journal, drivable by the
`meso` CLI (no MCP). It follows the **nomad** product model wholesale — Go API +
Go CLI + Vue, Authelia edge auth, registry-pull deploy.

- **What it is, why, and the domain model:** [`README.md`](README.md) — the README is the spec. Read it before adding an entity or endpoint.
- **Reference build to copy patterns from:** `~/webapps/nomad/` (`CLAUDE.md`, `api/`, `cli/`, `.planning/cli-auth-design.md`).
- **Universal rules live in `~/.claude/CLAUDE.md` and the fleet standards, and are not restated here.** `fleet standards applies meso` is which ones reach this repo — an enumeration in this line would drift, which is what the list that used to sit here did.
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

`internal/config` (OIDC/API settings, and `Config.Login()` mapping onto goclilogin) →
`internal/api` (typed REST client over the wire contract) → `internal/cli` (cobra command
tree). Resource commands live one file per resource under `internal/cli`, each with a
matching `internal/api/<resource>.go` client.

The device grant, the OS-keychain token store and the refresh are
`github.com/datapointchris/goclilogin`, shared with the icb, nomad and learning CLIs. Do
not reintroduce a local `internal/auth`: the library exists because four CLIs each had one
and a fix to any single copy left the other three broken. The refresh takes a machine-wide
lock, because Authelia revokes the whole grant when two processes replay the same rotated
refresh token.

Top-level commands are **training nouns only**, and anything about the software rather
than the training goes under `admin` (`internal/cli/admin.go`). The rule and the
HashiCorp precedent behind the verb are `standards/cli-design.md` § "`admin` is the
namespace for operating the app". `admin feedback` is its first inhabitant here; a new
non-domain command belongs there, not at the root.

**A command that cannot answer offers the commands that can.** What an error owes is a
fleet rule — `help.md` § "An error is the help screen for the failure in hand", whose
canonical source is the shared cobra bootstrap at `~/tools/goclikit`. What
is meso's own is the step past it: where the caller typed something valid and the CLI
still has nothing to show, the response is a list of runnable commands rather than a
failure at all. `resolveIDOrName` in `internal/cli/helpers.go` is the pattern. It writes
the menu itself and returns `exitCode`, which sets the process status without `Execute`
printing an `error:` line above it, and under `--json` it emits the same candidates as
data. Every command it composes takes its path from `cmd.CommandPath()` and its values
through `shellArg`, so a menu line can be pasted back as typed.

**It reaches `show` on movements, workouts and cycles, and nothing else yet.** `metrics
show`, `sessions show`, `log show` and every write verb still return plain errors through
`handleAPIError` and `usageArgs`. Widening it is a 33-site change to how arguments are
reported, and `help.md` puts that fix in `cobracmd` rather than per repo — so treat this
as describing three commands, not the CLI.

**A name is accepted where a command reads and never where it writes.** `show` resolves
`<id-or-name>`; `update`, `delete` and the composition verbs take ids. A fuzzy match that
narrows to the wrong record costs a wrong screen on a read and a wrong row on a write.

## Conventions specific to meso

Schema conventions (lookup tables not enums, `TEXT` columns, PK strategy, server-side filtering,
seed-carries-only-the-FK-backbone) and product independence are fleet standards — see
`standards/data.md` and `standards/api-design.md`. How they land here:

- **Lookup tables**: `movement_kinds`, `muscles`, `relationship_kinds`, `cycle_statuses`,
  `set_kinds`, `load_modes` — and those six are also exactly what `seed` carries. Everything else
  (metrics, movements, workouts, cycles, measurements, journal) loads through the `meso` CLI.
- **PKs**: catalog rows key on `.name` for the `UNIQUE` natural key, which is what lets a CLI
  import upsert. `workout_sessions` and `fitness_log_entries` are on UUID, and
  `session_movements` / `session_sets` are ordered-join rows.
- **Plan vs. performance**: `session_movements.target_*` is the prescription copied from the
  workout; `session_sets` is what was actually done. They are never the same columns again —
  merging them is what made a diverged session read as a failed one. This is the repo's own
  hard-won distinction and is in no standard.
- **Product independence** is `standards/api-design.md` § "Products depend on no sibling product",
  which takes `feedback` from this repo as its example: stored and triaged here, never forwarded.
  Don't re-propose a push integration.

The PK strategy, the `CHECK`-is-not-an-enum rule and the seed boundary are `standards/data.md`,
which drew several of its examples from this schema — so the standard is the copy of record and
the rules are not restated above, only the tables they land on.

`standards/data.md` § Known gaps records that `measurements` and `cycles` are on
`GENERATED ALWAYS AS IDENTITY` against the rule. That is still true here.

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
with a git-derived version embedded via ldflags. A locally built one reports as a dev
build and refuses to self-update — that is `standards/go.md` § "A module in a
subdirectory needs the full module path and a prefixed tag", not a defect to fix.
Released binaries come from CI on a `cli/v*` tag.
