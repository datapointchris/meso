# meso

A personal, mobile-first training app. A library of movements compiles into workouts; workouts sequence into goal-directed **cycles** (mesocycles); sessions are logged against them; stats and a journal are tracked — all drivable by Claude through the `meso` CLI so progress can be reviewed and the next cycle drafted collaboratively.

Self-hosted, single-user, no external users. Defined as much by what it leaves out — no videos, gamification, social feed, or motivation bloat — as by what it does.

**Status:** Phase 1 (Movements core) built locally; Phase 0 live in production. The unified movement library is complete end to end — Go API (`/api/v1/movements` CRUD + filtering, `/api/v1/muscles`), the `meso movements` CLI, and the Vue library/detail/add-edit surfaces, plus a seeded baseline catalog. The full design and phase plan live in [`docs/design.md`](docs/design.md); architecture and conventions in [`CLAUDE.md`](CLAUDE.md).

## Why "meso"

A _mesocycle_ is the unit of training planning: a multi-week block aimed at a target. The name is the thesis — the value is the planning intelligence, unified across strength, running, mobility, and dance, not tracking-for-tracking's-sake. It's the right internal name; it is deliberately not positioned as a commercial brand (see the name note in the design doc).

## Stack

A **product front door** in the sense of `~/dev/vision.md`, built on the **nomad** model:

- **Go** API — `pgx`, goose migrations, `net/http`, single binary.
- **Vue 3 + TypeScript** frontend — behind Authelia cookie SSO.
- **`meso` Go/cobra CLI** — a thin REST client and the agent surface. **No MCP** (the ecosystem is retiring MCP for CLI front doors).
- **Authelia** at the Traefik edge — web cookie SSO; CLI `meso auth login` via OAuth Auth-Code + PKCE, token in the OS keychain.
- **Registry-pull deploy** — ghcr images, `workflow_run` webhook, its own homelab host.

## Development

Bring up the full stack (Postgres + Go API + Caddy-served SPA) locally:

```bash
docker compose -f docker-compose.dev.yml up --build
# API  → http://localhost:8088/health
# web  → http://localhost:8080  (Caddy serves the SPA, proxies /api/* → API)
# seed the lookup tables once:
docker compose -f docker-compose.dev.yml run --rm --entrypoint ./meso-seed api
```

Or run pieces directly: the API with `cd api && go run .` (needs Postgres on `5459`), the SPA with `npm run dev` (Vite on `3001`, proxies `/api` → `8088`), and the CLI with `cd cli && go run . auth status`.

## Origin

Spun out of the ichrisbirch "Fitness Tracker" plan (2026-07-22). A different purpose and a hard mobile-first constraint warranted its own repo, and a clean break from ichrisbirch: meso is its own Go product modeled on nomad, not an ichrisbirch domain.
