# meso — design

A standalone, personal, mobile-first training app: a library of movements that compile into workouts, workouts that sequence into goal-directed **cycles** (mesocycles), sessions logged against them, tracked stats, and a journal — all drivable by Claude through a first-class `meso` CLI (no MCP) so progress can be reviewed and the next cycle drafted collaboratively.

The name is the thesis. A *mesocycle* is the unit of training planning — a multi-week block aimed at a target. The app's value is that planning intelligence, unified across strength, running, mobility, and dance, not tracking-for-tracking's-sake. The block entity is named **Cycle** in the model.

Replaces the loose markdown collection in `~/shart/fitness/` (transcribed workouts + three research-backed plans). See **Migration & Seed** for how that content becomes the initial catalog.

> **Why standalone, and modeled on nomad.** meso is a **product front door** in the sense of `~/dev/vision.md`: its own data store, its own web surface, and its own `meso` CLI — a clean-break app that does not depend on ichrisbirch. It follows the **nomad** product model wholesale (Go API + Go CLI + Vue, Authelia auth at the edge, registry-pull deploy), because nomad is the reference build for exactly this shape and copying its proven stack — including the fiddly CLI auth flow — beats reinventing it. It is the fourth product app (after `icb`, `learning`, `nomad`); the homelab is provisioned for the three, and meso needs its own slice (see Deployment).

> **No MCP — ever.** The entire ecosystem is retiring MCP in favor of CLI front doors (`~/dev/vision.md`, "CLI-primary and MCP retirement"). meso is built CLI-first from day one and never ships an MCP server. The `meso` CLI is the agent surface: Claude drives it via Bash.

> **Name note (settled 2026-07-22).** "meso" is the right *internal/personal* name — accurate and spare. It is a poor *commercial* brand: "mesocycle" is an industry term so the mark is descriptive/weak, and the fitness space already has several active "Meso"-named apps built on the same periodization thesis (meso.fitness, joinmeso.com, MesoBuilder, MesoTrack, Mesostrength) plus registered marks (MESO FIT / MESO METHOD, Class 41). None of that reaches a private, self-hosted, single-user tool. If commercial optionality is ever wanted, a distinct product/brand name would be chosen then while `meso` stays the codebase identity.

## Motivation

The markdown directory works as storage but not as a system. It can't answer "what do I do today," can't track that I did it, can't show a lift going up over three months, and can't be swapped/rotated without hand-editing files. The three Claude-authored plans (`shoulder-plan.md`, `lower-body-plan.md`, `dance-conditioning.md`) are genuinely researched and targeted, but they each define their own conflicting week and there's no way to reconcile them into one rotation without a real model underneath.

Two things make this worth building as an app rather than better markdown:

1. **Template vs. instance.** A workout is a plan; doing it on a Tuesday is an event with actual weights, checkboxes, and how it felt. Markdown collapses those. The whole point of tracking is the instance data.
2. **AI-assisted cycle drafting.** The differentiator. Because sessions, stats, and the journal are all reachable through the `meso` CLI, Claude can read the actual history — what got done, what stalled, how I felt — and help write the *next* cycle instead of me guessing. The capstone is a `meso review` read plus ordinary writes (below), not a server-side AI feature.

## Design philosophy — defined by what it leaves out

This app is defined by its exclusions. The value isn't more features; it's the absence of the bloat every commercial fitness app accretes. Stated up front so every future feature gets measured against it.

**What it is**: a digital tool for writing down exercises and making planned, informed training plans — the programming intelligence a bodybuilder applies to a mesocycle, or (especially) a 5k coach applies to a periodized race build, but unified across strength, running, yoga, stretching, and dance, with **multiple concurrent goals** running at once. The intelligence is in the *planning* — structured cycles toward real targets — not in tracking-for-tracking's-sake.

**What it is NOT** (hard exclusions — the anti-features list, reinforced by the competitive research below):

- No instructional videos, GIFs, or animations. Text how-to and form cues only (see Movement).
- No motivational content, quotes, coaching voice, or "you got this" copy.
- No gamification: no streaks, badges, points, levels, confetti, or achievement popups.
- No social feed, sharing, followers, or leaderboards.
- No calorie/macro/nutrition tracking creep.
- No ads, no upsell, no paywalled basics, no "AI coach" gimmickry (the AI here is a real reasoning surface over real data via the CLI, not a mascot).
- No forced onboarding, no notifications-by-default, no engagement mechanics.

The single-user, self-hosted, no-external-users nature is what makes this discipline free: there's no growth metric pushing bloat in. When in doubt about a feature, the default is **leave it out**.

**Multiple concurrent goals** is a first-class concept, not an afterthought: strength, a slow 5k build, mobility/flexibility, and dance conditioning progress in parallel, each with its own cycle and target metric, sharing one movement library and one weekly reality. The app's job is to hold several goals at once without making me pick one.

## Competitive landscape — where the wedge is

Research across strength loggers (Strong, Hevy, FitNotes, Jefit, Boostcamp), running planners (Couch-to-5k, Runna, TrainingPeaks, Garmin Coach, Nike Run Club), yoga/mobility (Down Dog, Yoga Studio, Alo Moves), and hybrids (Fitbod, Strava, Centr) surfaced one consistent gap that this build aims straight at. The mesocycle-programming niche specifically is now crowded (meso.fitness, MesoBuilder, MesoTrack, Mesostrength) — but every one of those is single-modality strength and closed/commercial; the unification + self-hosted + text-first + CLI-review combination remains the wedge.

**No incumbent unifies strength + endurance + mobility, and every one assumes a single active plan.** Serious hybrid athletes stitch two specialist apps together (Strava + Hevy, Runna + Strong) and become "the integration layer" themselves — "running apps don't know you lifted heavy yesterday." Modeling **multiple concurrent goals on one recovery-aware calendar** is the differentiator no one fills, which confirms the thesis above rather than contradicting it.

**Text-first movement libraries are underserved.** The best reference content (ExRx, Muscle & Strength, StrengthLog) is text — numbered steps, a discrete fault→fix mistakes list, honest primary/secondary muscle tagging — but most apps bury it in video. The stated need here (reconstruct a movement from writing, no video) is exactly the underserved slot.

**What users consistently reward** (folded into the entities and phases below): fastest-possible set logging with the previous session's numbers shown inline; a plate calculator; a rest timer that actually notifies and survives app-switching; PR/estimated-1RM and volume trends never paywalled; race-anchored periodization with a taper; **effort/HR-based intensity targets, not pace-only** (the universal "easy runs aren't easy" complaint); **adaptive rescheduling that slides the calendar** on a missed day instead of marking it failed; a "repeat this week until ready" safety valve; **legible cycles that explain why next week is what it is**; exercise substitution that **carries the weight/rep target to the swap**; and CSV export / offline operation (repeatedly named dealbreakers when absent).

**What users uninstall for** — this validates and extends the exclusions list above: social feeds, gamified streaks/badges, video dependency, chatty motivational voiceovers ("nails on a chalkboard"), opaque "AI coach" black boxes, paywalled basics, forced onboarding, calorie/macro creep, and choice-overload class libraries that answer "watch what?" instead of "do what today?" The through-line: the market's failure and user resentment live almost entirely in the excluded list, which is why "defined by what it leaves out" is the right organizing principle.

## Stack — the nomad product model

meso copies nomad's stack top to bottom. See `~/webapps/nomad/CLAUDE.md` for the reference build; this section states meso's version of it.

- **Frontend**: Vue 3 + TypeScript (Vite), Composition API (`<script setup>`), Vue Router SPA. Reuses the ichrisbirch **frontend** patterns that are language-agnostic to the backend — the SCSS/CSS-variable design system (so the design-style switcher stays feasible), theme composable, and the stats-kit components — since those are Vue, not Python. **Mobile-first is a hard requirement here** (see below). Web login is Authelia cookie SSO — no bespoke login page.
- **Backend API**: **Go** — single binary, multi-stage Alpine image. `pgx/v5` for Postgres; **goose** SQL migrations (DDL-only, run at startup); `net/http` ServeMux with method routing (`GET /api/v1/movements`); `slog` JSON logs to stdout. Handlers self-provision Postgres via **testcontainers** for integration tests. Internal service token for any service→service call; Authelia gates external traffic at the Traefik edge (no Authelia headers reach the API for `client_credentials` flows).
- **CLI**: **Go / cobra**, copied from nomad's `cli/` (`main.go` → `cli.Execute()`, `internal/cli` command tree, `internal/auth` OAuth flow + OS-keychain token store, `internal/config`). A **thin REST client** over the meso API, noun-first grammar (`meso movements list`), `--json` / exit-codes (0 ok, 1 runtime, 2 usage) / TTY-detection per the CLI ergonomics reference. **This is the agent + power-user surface — there is no MCP.**
- **Database**: PostgreSQL. `Text` columns only (never `varchar(n)`). Categorical fields use lookup tables with `Text PRIMARY KEY` + FK, never enums. UUID7 (`google/uuid` NewV7) for user-generated rows, Postgres `GENERATED` identity for catalog rows.
- **Deployment**: registry-pull, three… two images (web + api, **no mcp**) to `ghcr.io`, `workflow_run` webhook → its own homelab host. Details under **Deployment**.

Frontend container follows nomad: a Caddy image serving the built SPA with fallback and reverse-proxying `/api/*` → the Go API.

### Auth — Authelia at the edge (web cookie + CLI bearer)

Identical model to nomad; full design in `~/webapps/nomad/.planning/cli-auth-design.md`, Authelia itself in `~/homelab/containers/auth-lxc/README.md`.

- **Web**: Authelia **cookie SSO** at the Traefik edge. The browser redirects to `auth.ichrisbirch.com`, logs in once, and gets a session cookie. The Vue SPA stays on cookie auth; there is no app-issued JWT and no login page in meso itself. This is the "log in via the web" requirement.
- **CLI**: `meso auth login` runs the OAuth 2.0 **Authorization Code + PKCE + loopback** flow (RFC 8252) against Authelia as a **public client**, scopes `authelia.bearer.authz` + `offline_access`. The resulting token is **opaque**, authorized at the Traefik ForwardAuth **edge by audience** (`meso.ichrisbirch.com`) — the API does **not** self-validate JWTs. The token lives in the **OS keychain**, never on disk. Clients are registered **per (machine × app)** — `meso-cli-<host>`, derived from the short hostname — so a leaked token is audience-scoped and revocation is two-axis (machine, app). `meso auth` provides `login / logout / status --json / token` (the last for `curl -H "Authorization: Bearer $(meso auth token)"`).

This is not device flow, not a self-validating JWKS resource server, and not a reuse of any pre-existing JWT/PAT code — it is nomad's flow with the strings swapped.

## Domain Model

Six entities. The first three are the library and its compositions; the last three are the tracked reality.

```bash
Movement ──< WorkoutMovement >── Workout ──< CycleWorkout >── Cycle
   │                                 │                          │
   │ (self-ref: alternate/           │                          │ targets
   │  antagonist/progression)        ▼                          ▼
   │                            WorkoutSession ──< SessionMovement >   Goal (a Stat target)
   │                            (a workout done on a date)
   │
Measurement (time series: lift numbers, 5k time, toe reach, weight, ROM, ...)
FitnessLogEntry (dated journal, reviewed with Claude)
```

### 1. Movement — exercises, stretches, and yoga poses, unified

Exercises, stretches, and yoga poses are "the same concept." That's the design: **one `Movement` entity with a `movement_kind` lookup** (`exercise` | `stretch` | `yoga_pose`), not three near-identical tables. Kind-specific fields are nullable and used as appropriate (a pose has a Sanskrit name and a hold; a lift has a default rep scheme). This keeps favorites, tags, search, and relationships uniform across all three — you can build a workout mixing a squat, a couch stretch, and pigeon pose without crossing entity boundaries.

- `name`, `movement_kind` (FK lookup), `favorite` (bool), `rating` (1–5, nullable)
- `tags: Text[]` — the "good for" dimension (mobility, posterior-chain, desk-counter, anti-rotation, ...)
- `primary_muscles: Text[]`, `secondary_muscles: Text[]` — honest primary/secondary tagging (the ExRx distinction), which also drives substitution: an alternate must share the primary. (Lookup-backed vs. free tags — see Open Questions.)
- `equipment: Text[]` — barbell, dumbbell, mat, none
- `how_to: Text` (markdown) — how to perform it, step by step; for a sequence I like, the sequence written out
- `form_cues: Text` (markdown) — what to watch for to maintain good form
- `common_faults: Text` (markdown) — common failures / mistakes with this movement
- kind-specific nullable fields: `default_sets`, `default_reps`, `default_hold_seconds`, `sanskrit_name`
- `measurable_rom` (bool) — flags movements whose ROM is worth logging as a Measurement (e.g. knee-to-wall)
- `source_url`, `source_name` — where it came from (a plan, a video)
- `created_at`, `updated_at`

**The how-to fields are text, deliberately — no videos.** The need is "I forget exactly what a prisoner squat is, how to do it, what to watch for, and what commonly goes wrong." So a movement carries a written how-to, form cues, and common faults — enough to reconstruct the movement or explain a sequence I've built, nothing more. No instructional video, no GIF, no motivational framing (per the design philosophy). A prisoner-squat entry reads: *how-to* — hands laced behind the head, elbows wide, squat to depth keeping the chest up; *form cues* — elbows stay back, knees track over toes, weight mid-foot; *common faults* — elbows collapse forward, chest drops, heels lift. That's the whole ambition of the field.

### 2. Movement relationships — alternates, antagonists (self-ref)

Alternates and antagonist exercises are wanted. Modeled as a directional self-referential join, `movement_relationships(movement_id, related_movement_id, relationship_kind)` where kind ∈ `alternate` | `antagonist` | `progression` | `regression` | `see_also`. (Here `progression`/`regression` are the standard strength terms for a harder/easier *variant* of a movement — unrelated to the `Cycle` entity.)

The test for whether a relationship deserves a table (vs. being prose) is whether there's a genuine query behind it. Here there is: **swapping a movement for an alternate while building or rotating a workout is a concrete UI action** ("give me an alternate for barbell row that I have equipment for"). That query justifies the structure. Antagonist/progression/regression ride the same table for free.

Conservative fallback if we want to defer: express relationships as tags (`alt:barbell-row`) and add the join only when the swap UI is actually built. Recommendation is to build the self-ref in Phase 2, because the swap flow is core to the "rotate a main set + variety" goal — see Open Questions.

### 3. Workout — an ordered, themed composition of movements

A workout IS its ordered list of prescribed movements. This join is **payload-bearing and query-driven** (you render the workout from it), so the M2M-with-payload is correct here, not over-engineering.

- `Workout`: `name`, `theme: Text` (nullable — "push", "lower + shoulder rehab"), `tags: Text[]`, `notes: Text` (markdown), `favorite`, `estimated_minutes`, `created_at`, `updated_at`
- `WorkoutMovement` (ordered join): `workout_id`, `movement_id`, `position: Integer`, `sets`, `reps: Text` ("4–6", "AMRAP", "30s"), `load: Text` ("80% 1RM", "2 plates", "bodyweight"), `rest_seconds`, `superset_group: Text` (nullable), `notes: Text`
  - PK `(workout_id, position)` or surrogate id; ordered by `position`.

### 4. WorkoutSession — a workout performed on a date (the instance)

The "checkboxes that they are done" + "notes with a particular workout I do on a day." Distinct from the template so history is real data.

- `WorkoutSession`: `id` (UUID7 for user-created rows), `workout_id: FK` (nullable — allows an ad-hoc session), `performed_on: Date`, `duration_minutes`, `overall_notes: Text` (markdown), `felt: Text` (nullable energy/mood tag), `created_at`
- `SessionMovement` (per-exercise actuals): `session_id`, `movement_id`, `position`, `done: bool` (the checkbox), `actual_sets`, `actual_reps: Text`, `actual_load: Text`, `notes: Text`
  - Seeded from the workout's `WorkoutMovement` rows when a session is started from a template, then edited in place as it's performed.

The logging screen carries the features every strength user rewards (see Competitive landscape): the **previous session's actual weight/reps shown inline** next to each input so I know what to beat; a **plate calculator**; an optional **rest timer** that notifies and survives app-switching (off for supersets/circuits); and **set-type tags** (warmup / AMRAP / drop / failure). When a movement is swapped for an alternate mid-session, its target **carries over** to the substitute. This is the `ActiveSessionView` — the single most-used, most mobile-critical screen.

### 5. Cycle (mesocycle) — an ordered sequence of workouts toward a goal

The research-backed plans (12-week run return, the shoulder rehab arc) ARE cycles: workouts that flow over weeks toward a target. Named `Cycle` — echoing the app name and lifting vernacular; it names the table, routes, and CLI commands.

- `Cycle`: `name`, `goal_summary: Text`, `target_metric: Text` (nullable FK → a Measurement metric, e.g. "deadlift working weight"), `target_value`, `target_date: Date` (nullable — for race-anchored builds), `start_date`, `status: Text` (lookup: `planned` | `active` | `paused` | `complete`), `notes: Text`
- `CycleWorkout` (ordered join): `cycle_id`, `workout_id`, `position`, `week: Integer` (nullable), `phase: Text` (nullable — "base", "build", "taper"), `frequency: Text` ("2×/week"), `intensity: Text` (nullable — effort/HR target, not pace-only: "easy / Zone 2", per the "easy runs aren't easy" finding), `conditions: Text` (nullable — "when knee-to-wall symmetric, advance")

Two behaviors from the research make a cycle humane rather than a straightjacket — the top structural complaint about running apps:

- **Sliding reschedule, never "failed."** A missed session slides the calendar forward; it is never marked as a red failure. Cycles advance by readiness, not by wall-clock weekday.
- **"Repeat this week until ready"** — Couch-to-5k's most-loved safety valve. A week can be held/repeated (gated by its `conditions`) before advancing.
- **Legible, not opaque.** The cycle explains *why* the next block is what it is (from `phase` + `conditions` + the target), the opposite of the black-box adaptive plans people distrust. This is also what makes the AI-drafting capstone reviewable rather than magic.

### 6. Measurement — the tracked stats time series

Lift numbers, 5k time, toe reach, bodyweight, BMI, ROM — "all of those I can track ... that show progress." A metric definition + a value time series.

- `metric_definitions` (lookup + metadata): `name: Text PK` ("deadlift-working-weight", "5k-time", "knee-to-wall-left"), `unit: Text` ("lb", "seconds", "cm"), `direction: Text` (`higher_better` | `lower_better`), `category: Text` (strength | cardio | mobility | body)
- `measurements` (time series): `id`, `metric: FK → metric_definitions.name`, `value: Numeric`, `measured_on: Date`, `source: Text` (`manual` | `session` — some derive from logged sessions later), `notes: Text`

Feeds a stats page via the ported stats kit.

### 7. FitnessLogEntry — the journal

"A fitness log ... in addition to the comments on a particular workout ... write into the log and keep it for my journey" — the substrate Claude reviews.

- `FitnessLogEntry`: `id` (UUID7), `entry_date: Date`, `body: Text` (markdown), `tags: Text[]`, `mood: Text` (nullable), `created_at`, `updated_at`

## Design Decisions

- **Full nomad product model** — Go API + Go CLI + Vue, Authelia edge auth, registry-pull deploy. Copy the reference build rather than reinvent; consolidates the product tier (learning, nomad, meso) on Go.
- **No MCP** — the `meso` CLI is the sole programmatic/agent surface, in line with the ecosystem-wide MCP retirement.
- **One `Movement` entity, not three** — exercises/stretches/poses share every operation (favorite, tag, search, compose, relate). A `movement_kind` lookup + nullable kind-specific fields beats three parallel tables. (Confirm in Open Questions.)
- **Template vs. instance is the spine** — `Workout` (plan) and `WorkoutSession` (a dated performance) are separate entities. This is the single most important modeling call; it's what makes progress trackable instead of a pile of edited plans.
- **Workout composition is a real payload-bearing M2M** — `WorkoutMovement` carries order + prescription and is read on every render. The relationship *is* the query, so the M2M-with-payload is correct (as distinct from a prose relationship, which would not earn a table).
- **Movement relationships get a self-ref join, justified by the swap-alternate UI** — the concrete usage pattern. Falls back to tags if we defer (Open Questions).
- **Lookup tables, never enums** — `movement_kinds`, `metric_definitions`, `cycle_statuses`, `relationship_kinds` all `Text PRIMARY KEY` + FK. All text columns `Text`.
- **UUID7 PKs for user-generated rows** (`WorkoutSession`, `FitnessLogEntry`), identity for catalog rows (`Movement`, `Workout`) — keeps future client-side/offline creation feasible.
- **Go layering** — one file per resource across `handlers/` (HTTP), `repository/` (pgx queries), `models/` (domain structs), plus `middleware/` and `migrations/` (goose DDL), mirroring nomad's `api/` layout. Seed data is separate from migrations (a `cmd/seed` one-shot that UPSERTs).
- **Data ownership: CSV export from day one** — repeatedly named a dealbreaker when absent. `meso <resource> export --csv` is trivial for a single user and keeps the data portable.
- **Mobile-first is a hard requirement, not a polish pass** — see below.

### Mobile-first (a first-class constraint)

This is an iOS-in-the-gym app; mobile-first is the whole point, not a later pass. It's the app opened one-handed between sets. Requirements:

- Touch targets ≥ 44px; the session-logging screen (check off a set, bump a weight) must be usable one-handed without zoom.
- Responsive layouts from a phone viewport up — no horizontal scroll, no desktop-table-crammed-onto-phone.
- The "active session" view is the highest-traffic screen: big checkboxes, inline +/- for actual load/reps, minimal navigation.
- Consider a PWA/installable shell so it opens like an app and works with flaky gym wifi (offline session logging that syncs later — Open Question; UUID7 PKs keep it possible).
- The mobile patterns established here (breakpoints, touch components) should stay design-variable-driven so they compose with the design-style-switcher idea.

## Data-model sketch

snake_case tables, `Text` columns, lookup tables for all categoricals, goose migrations (DDL-only). Abbreviated:

```bash
movement_kinds(name PK)                                   -- exercise|stretch|yoga_pose
relationship_kinds(name PK)                               -- alternate|antagonist|progression|regression|see_also
cycle_statuses(name PK)                                   -- planned|active|paused|complete
metric_definitions(name PK, unit, direction, category)

movements(id PK identity, name, movement_kind FK, favorite,
          rating, tags Text[], primary_muscles Text[], secondary_muscles Text[],
          equipment Text[], how_to, form_cues, common_faults,
          default_sets, default_reps, default_hold_seconds, sanskrit_name,
          measurable_rom, source_url, source_name, created_at, updated_at)

movement_relationships(movement_id FK, related_movement_id FK,
          relationship_kind FK, PK(movement_id, related_movement_id, relationship_kind),
          CHECK movement_id != related_movement_id)

workouts(id PK identity, name, theme, tags Text[], notes,
          favorite, estimated_minutes, created_at, updated_at)
workout_movements(id PK, workout_id FK, movement_id FK, position,
          sets, reps, load, rest_seconds, superset_group, notes)

workout_sessions(id PK uuid7, workout_id FK NULL, performed_on,
          duration_minutes, overall_notes, felt, created_at)
session_movements(id PK, session_id FK, movement_id FK, position,
          done bool, actual_sets, actual_reps, actual_load, notes)

cycles(id PK identity, name, goal_summary, target_metric FK NULL,
          target_value, target_date NULL, start_date, status FK, notes)
cycle_workouts(id PK, cycle_id FK, workout_id FK, position,
          week, phase, frequency, intensity, conditions)

measurements(id PK, metric FK, value Numeric, measured_on, source, notes)

fitness_log_entries(id PK uuid7, entry_date, body, tags Text[],
          mood, created_at, updated_at)
```

## API request/response types

Go request/response structs per entity (Create / response / Update shapes) with JSON tags; the DB owns computed/derived values (e.g. `created_at`, server-stamped timestamps), never the client. Detail responses embed relations:

- `WorkoutWithMovements` — `GET /api/v1/workouts/{id}`, embeds ordered `WorkoutMovement` + each `Movement` summary.
- `WorkoutSessionWithMovements` — embeds `SessionMovement` rows.
- `CycleWithWorkouts` — embeds the ordered workout sequence.
- `MovementWithRelations` — embeds alternates/antagonists as `Movement` summaries.

## API endpoint shape (RESTful, `net/http` ServeMux)

```sql
/api/v1/movements                 CRUD + ?kind=&favorite=&tag=&equipment=&search=
/api/v1/movements/{id}/related    POST {related_movement_id, relationship_kind}; DELETE .../{rid}
/api/v1/workouts                  CRUD + ?theme=&tag=&favorite=
/api/v1/workouts/{id}/movements   POST add movement; PATCH reorder/prescription; DELETE remove
/api/v1/sessions                  CRUD; POST from ?workout_id= copies template into session
/api/v1/sessions/{id}/movements/{mid}   PATCH done/actuals
/api/v1/cycles                    CRUD
/api/v1/cycles/{id}/workouts      POST/PATCH/DELETE ordered workouts
/api/v1/metrics                   list metric_definitions; POST define
/api/v1/measurements              CRUD + ?metric=&from=&to=
/api/v1/log                       CRUD dated entries
/api/v1/stats                     aggregations for the stats page
/api/v1/review                    GET structured recent history (sessions+measurements+log) for `meso review`
/health                           unauthenticated liveness (exempt from edge auth)
```

Every endpoint group ships with its goose migration, a seed contribution, and a testcontainers-backed handler test.

## CLI — the tool surface (replaces MCP)

`meso` is a Go/cobra thin REST client over the API, the agent + power-user surface. Every read supports `--json` (stable schema, for scripting and for Claude to parse); writes echo the created/updated row. Noun-first grammar, one command group per resource:

```bash
meso auth        login | logout | status [--json] | token          # nomad's flow, verbatim
meso movements   list [--kind --favorite --tag --equipment --search] | show <id> | create | update | delete | related add/rm | export --csv
meso workouts    list | show <id> | create | update | delete | movements add/reorder/rm
meso sessions    log [--from-workout <id>] | list [--from --to] | show <id> | movement done <mid>
meso cycles      list | show <id> | create | update | delete | workouts add/reorder/rm
meso metrics     list | define
meso measurements record | list [--metric --from --to] | trend <metric>
meso log         add | list [--from --to --tag] | show <id>
meso stats       [--json]
meso review      [--since 30d] [--json]      # the capstone read
```

**The AI capstone is a read plus ordinary writes — no server-side LLM.** `meso review --json` pulls recent sessions + measurements + log into one structured payload. Claude reads it via Bash, reasons about the next block *in the conversation*, and persists the drafted cycle with ordinary writes (`meso cycles create`, `meso cycles workouts add …`). This is strictly simpler than an MCP `draft_cycle` tool: the CLI only needs solid read + write primitives; Claude is the reasoning.

## Frontend (Vue) — top nav

```text
Workouts  |  Movements  |  Cycles  |  Log  |  Stats
```

Views: `MovementsView` (filterable library, favorites), `MovementDetailView` (how-to markdown, alternates/antagonists, "in these workouts"), `WorkoutsView`, `WorkoutDetailView` (ordered movements, "start session" button), `ActiveSessionView` (the mobile-critical logging screen), `CyclesView` + detail (week/phase timeline), `FitnessLogView` (dated markdown entries), `StatsView` (ported stats kit). Each `AddEdit*Modal` wrapper per the per-entity convention. Markdown via a shared `markdown-it` util. Web auth is Authelia cookie SSO at the edge — no login view in the app.

## Stats page

Ported stats kit (`StatsSummaryCards`, `StatsTable`, `useStatsCharts`, theme-driven colors). Candidate charts: strength metrics over time (line, per lift), 5k time trend, mobility ROM trend (knee-to-wall, toe reach), session frequency by week, movements-by-kind mix, favorites coverage. All colors via theme CSS custom properties — no hardcoding.

## Deployment — nomad's registry-pull pattern

Push to `main` → GitHub Actions runs lint + Go tests + Vue tests → builds **two** Docker images (`meso-web`, `meso-api`; **no mcp**) and pushes to `ghcr.io/datapointchris/meso-{web,api}` with `:latest` and `:sha-<commit>` → `workflow_run.completed` webhook fires a `deploy-meso.sh` on app-ops-lxc → SSH to the meso host → `git pull && docker compose pull && docker compose up -d --no-build`. No on-host compilation.

Traefik on app-ops routes `meso.ichrisbirch.com` to the meso host's `web` container with the Authelia middleware. The web container (Caddy) serves the SPA and proxies `/api/*` → the Go API. Production-quality baked in from the start, per nomad: `/health` on web + api (unauthenticated), SIGTERM-graceful shutdown, structured JSON logs to stdout scraped by Promtail into Loki, SHA-pinned image tags.

### Homelab provisioning — meso as "a new one"

The homelab is set up for the three (`icb`, `learning`, `nomad`); meso needs its own slice, mirroring nomad's setup (reference: `~/webapps/nomad/.planning/cli-auth-design.md`, `~/homelab/containers/auth-lxc/README.md`):

- A **host** for the app (its own LXC, or a shared app host — Open Question), plus a Postgres for meso.
- **Authelia clients**: one public CLI client per machine, `meso-cli-<host>`, with loopback redirect URIs; and the `meso.ichrisbirch.com` audience wired into the ForwardAuth edge.
- **Traefik**: a `meso.ichrisbirch.com` router → meso web container, with the Authelia middleware.
- **CI/CD**: a `ghcr.io/datapointchris/meso-*` image repo and a `deploy-meso.sh` + webhook route on app-ops-lxc.

## Migration & Seed

The `~/shart/fitness/` content becomes the initial catalog, so the app isn't empty on first boot:

- **Movements**: the exercises across `ppl/*.md`, `general-workouts/apartment-*.md`, `michael-workout-*.md`, `flexibility-moves.md`, plus the movements named in the three research plans → `movements` rows with tags/muscles.
- **Workouts**: each `ppl/*.md` sheet, each apartment/michael workout, and the plans' named sessions (A/B/C/D) → `workouts` with ordered `workout_movements` carrying the transcribed sets/reps/load.
- **Cycles**: `lower-body-plan.md` (12-week run return) and `shoulder-plan.md` (rehab arc) → `cycles` sequencing their sessions; `dance-conditioning.md` shelved as a `paused` cycle.
- **Goals/metrics**: seed `metric_definitions` from `shart/fitness/goals.md` (deadlift working weight, 5k time, knee-to-wall L/R, toe reach, bodyweight) and `program.md`'s Week-0 baseline test as the first `measurements` once recorded.

Mechanism: a `cmd/seed` one-shot for the lookup tables and a baseline catalog, then a **CLI-driven import pass** (Claude reads the markdown and calls `meso movements create` / `meso workouts create` etc.) for the richer content — exercising the CLI against real content, exactly the kind of validation that shakes out model rough edges before automation. Decide the split during Phase 1.

## Phased plan

Each phase independently shippable.

- **Phase 0 — Scaffold.** Repo → running app: Go API skeleton (`net/http`, pgx, goose, `/health`), Postgres, Vue shell, Caddy web image, Docker compose, the **Go CLI skeleton copied from nomad** with `meso auth login/logout/status/token` working against Authelia, homelab provisioning (Authelia clients, `meso.ichrisbirch.com` route, ghcr repo, deploy webhook), CI. *Acceptance*: `meso auth login` succeeds and `meso auth status --json` reports logged-in; an empty Vue shell loads behind Authelia SSO; `/health` green.
- **Phase 1 — Movements core.** `Movement` + kind/relationship lookups, CRUD, search/filter, favorites, CLI `meso movements …`, Vue library + detail + modal, **mobile-first patterns established here**, seed. Import shart movements. *Acceptance*: browse/search/favorite a unified library of exercises, stretches, and poses on a phone; `meso movements list --json` round-trips.
- **Phase 2 — Workouts + relationships.** `Workout` + `WorkoutMovement` ordered composition, `movement_relationships` self-ref + swap-alternate UI, CLI. Import shart workouts. *Acceptance*: build/compose a themed workout, swap a movement for an alternate.
- **Phase 3 — Sessions (logging).** `WorkoutSession` + `SessionMovement`, start-from-template, checkboxes, actuals, per-session notes. The `ActiveSessionView`. *Acceptance*: do a workout on the phone, check off sets, record real weights, add a note.
- **Phase 4 — Measurements + Stats.** Metric definitions, measurement logging, `StatsView` with the ported kit. *Acceptance*: log a lift/ROM/time and see the trend chart.
- **Phase 5 — Fitness log.** Dated journal entries, CLI read/write. *Acceptance*: write and browse journal entries; `meso log list --json` works.
- **Phase 6 — Cycles + AI drafting.** `Cycle` sequencing, `meso review` capstone read. Import the research plans as cycles. *Acceptance*: an active cycle drives "what's next," and Claude drafts the following cycle from real `meso review` history and persists it via `meso cycles` writes.

## Open questions (before/while building)

1. **Unified `Movement` entity — agree?** Recommended (one entity + kind). Only reason to split is if poses/stretches diverge far more than expected.
2. **Movement relationships now or later?** Recommended in Phase 2 (self-ref, because swap-alternate is a core goal). Defer-to-tags is the conservative alternative.
3. **Muscle/region tagging — lookup table or free tags?** Lookup gives clean filtering and a body-map UI later; tags are lighter. Leaning lookup, since "show me posterior-chain movements" is a real filter.
4. **Offline session logging (PWA)?** Gym wifi is unreliable. UUID7 PKs keep offline-create possible, but full offline sync is a large add — worth it, or is a good responsive web app enough for v1?
5. **Host: own LXC or a shared app host?** nomad has its own `nomad-lxc`. meso could get `meso-lxc` or share an app host. Decide in Phase 0 provisioning.

### Settled

- **Stack: full nomad model** — Go API + Go CLI + Vue, Authelia edge auth (web cookie / CLI PKCE), registry-pull deploy. No MCP.
- **Block entity named `Cycle`** (not Progression / Mesocycle).
- **Web login = Authelia cookie SSO**; CLI login = `meso auth login` (OAuth PKCE, keychain token).
- **Private-only** — single user, no multi-tenant scope.

## Out of scope (v1)

- Wearable/health-app import (Apple Health, Strava) — later.
- Auto-deriving lift stats from session data — measurements are manual in v1; the `source` field leaves the door open.
- Video attachments for movements — `source_url` link only.
- Social/sharing — private, like the rest of the ecosystem.

## References

- Reference build (stack + CLI + deploy to copy): `~/webapps/nomad/` — especially `CLAUDE.md`, `cli/`, and `.planning/cli-auth-design.md`.
- Cross-system architecture this app slots into: `~/dev/vision.md` (product front doors, CLI-primary, MCP retirement) and `~/dev/system.md`.
- Authelia / homelab auth: `~/homelab/containers/auth-lxc/README.md`.
- Source content to migrate: `~/shart/fitness/` — `ppl/*.md`, `general-workouts/apartment-*.md`, `michael-workout-*.md`, `simple-machine-workout.md`, `flexibility-moves.md`, `deadpool-2-ab-workout.md`, `ryan-reynolds-ripped.md`, and the three research plans `shoulder-plan.md`, `lower-body-plan.md`, `dance-conditioning.md`.
- Goals & rotation the app formalizes: `~/shart/fitness/goals.md`, `~/shart/fitness/program.md`.
