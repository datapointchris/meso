# meso

A personal, mobile-first training app. A library of movements compiles into workouts; workouts sequence into goal-directed **mesocycles**; sessions are logged against them; stats and a journal are tracked — all readable by Claude via MCP so progress can be reviewed and the next cycle drafted collaboratively.

Self-hosted, single-user, no external users. Defined as much by what it leaves out — no videos, gamification, social feed, or motivation bloat — as by what it does.

**Status:** design stage. No code yet. The full design lives in [`docs/design.md`](docs/design.md).

## Why "meso"

A *mesocycle* is the unit of training planning: a multi-week block aimed at a target. The name is the thesis — the value is the planning intelligence, unified across strength, running, mobility, and dance, not tracking-for-tracking's-sake. It's the right internal name; it is deliberately not positioned as a commercial brand (see the name note in the design doc).

## Origin

Spun out of the ichrisbirch "Fitness Tracker" plan — a different purpose and mobile-first constraint warranted its own repo, deployment, and MCP server. Reuses the ichrisbirch stack (FastAPI + Vue 3 + PostgreSQL + FastMCP, Docker/Traefik on the homelab) as an independent app.
