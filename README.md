# meso

A personal, mobile-first training app. A library of movements compiles into workouts; workouts sequence into goal-directed **cycles** (mesocycles); sessions are logged against them; stats and a journal are tracked — all drivable by Claude through the `meso` CLI so progress can be reviewed and the next cycle drafted collaboratively.

Self-hosted, single-user, no external users. Defined as much by what it leaves out — no videos, gamification, social feed, or motivation bloat — as by what it does.

**Status:** design stage. No code yet. The full design lives in [`docs/design.md`](docs/design.md).

## Why "meso"

A *mesocycle* is the unit of training planning: a multi-week block aimed at a target. The name is the thesis — the value is the planning intelligence, unified across strength, running, mobility, and dance, not tracking-for-tracking's-sake. It's the right internal name; it is deliberately not positioned as a commercial brand (see the name note in the design doc).

## Stack

A **product front door** in the sense of `~/dev/vision.md`, built on the **nomad** model:

- **Go** API — `pgx`, goose migrations, `net/http`, single binary.
- **Vue 3 + TypeScript** frontend — behind Authelia cookie SSO.
- **`meso` Go/cobra CLI** — a thin REST client and the agent surface. **No MCP** (the ecosystem is retiring MCP for CLI front doors).
- **Authelia** at the Traefik edge — web cookie SSO; CLI `meso auth login` via OAuth Auth-Code + PKCE, token in the OS keychain.
- **Registry-pull deploy** — ghcr images, `workflow_run` webhook, its own homelab host.

## Origin

Spun out of the ichrisbirch "Fitness Tracker" plan (2026-07-22). A different purpose and a hard mobile-first constraint warranted its own repo, and a clean break from ichrisbirch: meso is its own Go product modeled on nomad, not an ichrisbirch domain.
