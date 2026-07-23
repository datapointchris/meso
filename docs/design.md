# meso — design

A standalone, personal, mobile-first training app: a library of movements that compile into workouts, workouts that sequence into goal-directed **mesocycles**, sessions logged against them, tracked stats, and a journal — all readable by Claude via MCP so progress can be reviewed and the next cycle drafted collaboratively.

The name is the thesis. A *mesocycle* is the unit of training planning — a multi-week block aimed at a target. The app's value is that planning intelligence, unified across strength, running, mobility, and dance, not tracking-for-tracking's-sake.

Replaces the loose markdown collection in `~/shart/fitness/` (transcribed workouts + three research-backed plans). See **Migration & Seed** for how that content becomes the initial catalog.

> **Why standalone, not a domain inside ichrisbirch.** This app has a fundamentally different purpose and audience-of-use than ichrisbirch's life-management surface: it's the thing opened one-handed at the gym, mobile-first as a hard requirement, defined by a tight exclusions list. It gets its own repo, its own deployment, and its own MCP server so its identity and constraints stay clean. It reuses ichrisbirch's proven stack and conventions (below) but as an independent app.

> **Name note (settled 2026-07-22).** "meso" is the right *internal/personal* name — accurate and spare. It is a poor *commercial* brand: "mesocycle" is an industry term so the mark is descriptive/weak, and the fitness space already has several active "Meso"-named apps built on the same periodization thesis (meso.fitness, joinmeso.com, MesoBuilder, MesoTrack, Mesostrength) plus registered marks (MESO FIT / MESO METHOD, Class 41). None of that reaches a private, self-hosted, single-user tool. If commercial optionality is ever wanted, a distinct product/brand name would be chosen then while `meso` stays the codebase identity.

## Motivation

The markdown directory works as storage but not as a system. It can't answer "what do I do today," can't track that I did it, can't show a lift going up over three months, and can't be swapped/rotated without hand-editing files. The three Claude-authored plans (`shoulder-plan.md`, `lower-body-plan.md`, `dance-conditioning.md`) are genuinely researched and targeted, but they each define their own conflicting week and there's no way to reconcile them into one rotation without a real model underneath.

Two things make this worth building as an app rather than better markdown:

1. **Template vs. instance.** A workout is a plan; doing it on a Tuesday is an event with actual weights, checkboxes, and how it felt. Markdown collapses those. The whole point of tracking is the instance data.
2. **AI-assisted progression.** The differentiator. Because sessions, stats, and the journal all live behind MCP, Claude can read the actual history — what got done, what stalled, how I felt — and help write the *next* cycle instead of me guessing. This is the fitness analog of the recipes URL-ingestion feature that worked well in ichrisbirch: the capability that justifies the build.

## Design philosophy — defined by what it leaves out

This app is defined by its exclusions. The value isn't more features; it's the absence of the bloat every commercial fitness app accretes. Stated up front so every future feature gets measured against it.

**What it is**: a digital tool for writing down exercises and making planned, informed training plans — the programming intelligence a bodybuilder applies to a mesocycle, or (especially) a 5k coach applies to a periodized race build, but unified across strength, running, yoga, stretching, and dance, with **multiple concurrent goals** running at once. The intelligence is in the *planning* — structured progressions toward real targets — not in tracking-for-tracking's-sake.

**What it is NOT** (hard exclusions — the anti-features list, reinforced by the competitive research below):

- No instructional videos, GIFs, or animations. Text how-to and form cues only (see Movement).
- No motivational content, quotes, coaching voice, or "you got this" copy.
- No gamification: no streaks, badges, points, levels, confetti, or achievement popups.
- No social feed, sharing, followers, or leaderboards.
- No calorie/macro/nutrition tracking creep.
- No ads, no upsell, no paywalled basics, no "AI coach" gimmickry (the AI here is a real reasoning surface over real data, not a mascot).
- No forced onboarding, no notifications-by-default, no engagement mechanics.

The single-user, self-hosted, no-external-users nature is what makes this discipline free: there's no growth metric pushing bloat in. When in doubt about a feature, the default is **leave it out**.

**Multiple concurrent goals** is a first-class concept, not an afterthought: strength, a slow 5k build, mobility/flexibility, and dance conditioning progress in parallel, each with its own progression and target metric, sharing one movement library and one weekly reality. The app's job is to hold several goals at once without making me pick one.

## Competitive landscape — where the wedge is

Research across strength loggers (Strong, Hevy, FitNotes, Jefit, Boostcamp), running planners (Couch-to-5k, Runna, TrainingPeaks, Garmin Coach, Nike Run Club), yoga/mobility (Down Dog, Yoga Studio, Alo Moves), and hybrids (Fitbod, Strava, Centr) surfaced one consistent gap that this build aims straight at. Note that the mesocycle-programming niche specifically is now crowded (meso.fitness, MesoBuilder, MesoTrack, Mesostrength) — but every one of those is single-modality strength and closed/commercial; the unification + self-hosted + text-first + MCP-review combination remains the wedge.

**No incumbent unifies strength + endurance + mobility, and every one assumes a single active plan.** Serious hybrid athletes stitch two specialist apps together (Strava + Hevy, Runna + Strong) and become "the integration layer" themselves — "running apps don't know you lifted heavy yesterday." Modeling **multiple concurrent goals on one recovery-aware calendar** is the differentiator no one fills, which confirms the thesis above rather than contradicting it.

**Text-first movement libraries are underserved.** The best reference content (ExRx, Muscle & Strength, StrengthLog) is text — numbered steps, a discrete fault→fix mistakes list, honest primary/secondary muscle tagging — but most apps bury it in video. The stated need here (reconstruct a movement from writing, no video) is exactly the underserved slot.

**What users consistently reward** (folded into the entities and phases below): fastest-possible set logging with the previous session's numbers shown inline; a plate calculator; a rest timer that actually notifies and survives app-switching; PR/estimated-1RM and volume trends never paywalled; race-anchored periodization with a taper; **effort/HR-based intensity targets, not pace-only** (the universal "easy runs aren't easy" complaint); **adaptive rescheduling that slides the calendar** on a missed day instead of marking it failed; a "repeat this week until ready" safety valve; **legible progression that explains why next week is what it is**; exercise substitution that **carries the weight/rep target to the swap**; and CSV export / offline operation (repeatedly named dealbreakers when absent).

**What users uninstall for** — this validates and extends the exclusions list above: social feeds, gamified streaks/badges, video dependency, chatty motivational voiceovers ("nails on a chalkboard"), opaque "AI coach" black boxes, paywalled basics, forced onboarding, calorie/macro creep, and choice-overload class libraries that answer "watch what?" instead of "do what today?" The through-line: the market's failure and user resentment live almost entirely in the excluded list, which is why "defined by what it leaves out" is the right organizing principle.

## Stack & conventions

meso inherits ichrisbirch's proven stack and playbook — this is deliberate reuse of a known-good foundation, deployed as its own independent app on the homelab.

- **Backend**: FastAPI + SQLAlchemy 2.0 (`Mapped[type]` declarative), Pydantic v2 schemas (`*Create` / base / `*Update` trio, `ConfigDict(from_attributes=True)`), Alembic migrations. Package management via uv.
- **Frontend**: Vue 3 + TypeScript SPA, Pinia stores with structured logging + `ApiError` handling, the shared SCSS/CSS-variable design system (so a future design-style switcher stays feasible). **Mobile-first is a hard requirement here from day one** (see below).
- **Database**: PostgreSQL. `Text` columns only (never `String(n)`). Categorical fields use lookup tables with `Text PRIMARY KEY` + FK, never enums. UUID7 PKs for user-generated rows, Integer identity for catalog rows.
- **AI surface**: a dedicated FastMCP server (streamable HTTP in prod, stdio in dev) — the read+write tool surface Claude uses to review and draft.
- **Deployment**: Docker Compose + Traefik on the homelab, its own containers/routes independent of ichrisbirch.
- **Per-entity component convention**: every reusable pattern (e.g. an `AddEditModal`) gets a per-entity wrapper — consistency over convenience, same as ichrisbirch.
- **Mandatory-checklist discipline**: every new API endpoint group ships with model + migration + schemas + router + **seeder** + test data + API tests.

Open: whether to stand up the stats/charts kit fresh or port ichrisbirch's shared stats components (`StatsSummaryCards`, `StatsTable`, `useStatsCharts`, theme-driven colors). Porting is the likely path — it's already theme-variable-driven.

## Domain Model

Six entities. The first three are the library and its compositions; the last three are the tracked reality.

```bash
Movement ──< WorkoutMovement >── Workout ──< ProgressionWorkout >── Progression
   │                                 │                                   │
   │ (self-ref: alternate/           │                                   │ targets
   │  antagonist/progression)        ▼                                   ▼
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

Alternates and antagonist exercises are wanted. Modeled as a directional self-referential join, `movement_relationships(movement_id, related_movement_id, relationship_kind)` where kind ∈ `alternate` | `antagonist` | `progression` | `regression` | `see_also`.

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

### 5. Progression (mesocycle) — an ordered sequence of workouts toward a goal

The research-backed plans (12-week run return, the shoulder rehab arc) ARE progressions: workouts that flow over weeks toward a target. The entity name — `Progression` — is settled here even though the app is *called* meso; a mesocycle is the domain concept, `Progression` is the table. (Naming revisit is an Open Question.)

- `Progression`: `name`, `goal_summary: Text`, `target_metric: Text` (nullable FK → a Measurement metric, e.g. "deadlift working weight"), `target_value`, `target_date: Date` (nullable — for race-anchored builds), `start_date`, `status: Text` (lookup: `planned` | `active` | `paused` | `complete`), `notes: Text`
- `ProgressionWorkout` (ordered join): `progression_id`, `workout_id`, `position`, `week: Integer` (nullable), `phase: Text` (nullable — "base", "build", "taper"), `frequency: Text` ("2×/week"), `intensity: Text` (nullable — effort/HR target, not pace-only: "easy / Zone 2", per the "easy runs aren't easy" finding), `conditions: Text` (nullable — "when knee-to-wall symmetric, advance")

Two behaviors from the research make a progression humane rather than a straightjacket — the top structural complaint about running apps:

- **Sliding reschedule, never "failed."** A missed session slides the calendar forward; it is never marked as a red failure. Progressions advance by readiness, not by wall-clock weekday.
- **"Repeat this week until ready"** — Couch-to-5k's most-loved safety valve. A week can be held/repeated (gated by its `conditions`) before advancing.
- **Legible, not opaque.** The progression explains *why* the next block is what it is (from `phase` + `conditions` + the target), the opposite of the black-box adaptive plans people distrust. This is also what makes the AI-drafting capstone reviewable rather than magic.

### 6. Measurement — the tracked stats time series

Lift numbers, 5k time, toe reach, bodyweight, BMI, ROM — "all of those I can track ... that show progress." A metric definition + a value time series.

- `metric_definitions` (lookup + metadata): `name: Text PK` ("deadlift-working-weight", "5k-time", "knee-to-wall-left"), `unit: Text` ("lb", "seconds", "cm"), `direction: Text` (`higher_better` | `lower_better`), `category: Text` (strength | cardio | mobility | body)
- `measurements` (time series): `id`, `metric: FK → metric_definitions.name`, `value: Numeric`, `measured_on: Date`, `source: Text` (`manual` | `session` — some derive from logged sessions later), `notes: Text`

Feeds a stats page via the shared stats kit.

### 7. FitnessLogEntry — the journal

"A fitness log ... in addition to the comments on a particular workout ... write into the log and keep it for my journey" — the substrate Claude reviews.

- `FitnessLogEntry`: `id` (UUID7), `entry_date: Date`, `body: Text` (markdown), `tags: Text[]`, `mood: Text` (nullable), `created_at`, `updated_at`

## Design Decisions

- **One `Movement` entity, not three** — exercises/stretches/poses share every operation (favorite, tag, search, compose, relate). A `movement_kind` lookup + nullable kind-specific fields beats three parallel tables. (Confirm in Open Questions.)
- **Template vs. instance is the spine** — `Workout` (plan) and `WorkoutSession` (a dated performance) are separate entities. This is the single most important modeling call; it's what makes progress trackable instead of a pile of edited plans.
- **Workout composition is a real payload-bearing M2M** — `WorkoutMovement` carries order + prescription and is read on every render. The relationship *is* the query, so the M2M-with-payload is correct (as distinct from a prose relationship, which would not earn a table).
- **Movement relationships get a self-ref join, justified by the swap-alternate UI** — the concrete usage pattern. Falls back to tags if we defer (Open Questions).
- **Lookup tables, never enums** — `movement_kinds`, `metric_definitions`, `progression_statuses`, `relationship_kinds` all `Text PRIMARY KEY` + FK. All text columns `Text`, never `String(n)`.
- **UUID7 PKs for user-generated rows** (`WorkoutSession`, `FitnessLogEntry`), Integer identity for catalog rows (`Movement`, `Workout`) — keeps future client-side/offline creation feasible.
- **Domain co-location** — everything lives in `models/fitness.py`, `schemas/fitness.py`, `api/endpoints/fitness.py` (or the flat equivalent for a single-domain app), routes under `/fitness/...` or the app root.
- **Data ownership: CSV export from day one** — repeatedly named a dealbreaker when absent. Self-hosted already means the data is mine, but a plain export keeps it portable and is trivial for a single user.
- **Mobile-first is a hard requirement, not a polish pass** — see below.

### Mobile-first (a first-class constraint)

This is an iOS-in-the-gym app; mobile-first is the whole point, not a later pass. It's the app opened one-handed between sets. Requirements:

- Touch targets ≥ 44px; the session-logging screen (check off a set, bump a weight) must be usable one-handed without zoom.
- Responsive layouts from a phone viewport up — no horizontal scroll, no desktop-table-crammed-onto-phone.
- The "active session" view is the highest-traffic screen: big checkboxes, inline +/- for actual load/reps, minimal navigation.
- Consider a PWA/installable shell so it opens like an app and works with flaky gym wifi (offline session logging that syncs later — Open Question; UUID7 PKs keep it possible).
- The mobile patterns established here (breakpoints, touch components) should stay design-variable-driven so they compose with the design-style-switcher idea.

## Data-model sketch

snake_case tables, `Text` columns, lookup tables for all categoricals. Abbreviated:

```bash
movement_kinds(name PK)                                   -- exercise|stretch|yoga_pose
relationship_kinds(name PK)                               -- alternate|antagonist|progression|regression|see_also
progression_statuses(name PK)                             -- planned|active|paused|complete
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

progressions(id PK identity, name, goal_summary, target_metric FK NULL,
          target_value, target_date NULL, start_date, status FK, notes)
progression_workouts(id PK, progression_id FK, workout_id FK, position,
          week, phase, frequency, intensity, conditions)

measurements(id PK, metric FK, value Numeric, measured_on, source, notes)

fitness_log_entries(id PK uuid7, entry_date, body, tags Text[],
          mood, created_at, updated_at)
```

## Pydantic schemas

`*Create` / base / `*Update` trio per entity (`ConfigDict(from_attributes=True)`). Detail schemas that embed relations:

- `WorkoutWithMovements` — `GET /workouts/{id}/`, embeds ordered `WorkoutMovement` + each `Movement` summary.
- `WorkoutSessionWithMovements` — embeds `SessionMovement` rows.
- `ProgressionWithWorkouts` — embeds the ordered workout sequence.
- `MovementWithRelations` — embeds alternates/antagonists as `Movement` summaries.

## API endpoint shape (RESTful sub-resources)

```sql
/movements/                 CRUD + ?kind=&favorite=&tag=&equipment=&search=
/movements/{id}/related/    POST {related_movement_id, relationship_kind}; DELETE .../{rid}
/workouts/                  CRUD + ?theme=&tag=&favorite=
/workouts/{id}/movements/   POST add movement; PATCH reorder/prescription; DELETE remove
/sessions/                  CRUD; POST from ?workout_id= copies template into session
/sessions/{id}/movements/{mid}   PATCH done/actuals
/progressions/              CRUD
/progressions/{id}/workouts/     POST/PATCH/DELETE ordered workouts
/metrics/                   list metric_definitions; POST define
/measurements/              CRUD + ?metric=&from=&to=
/log/                       CRUD dated entries
/stats/                     aggregations for the stats page
```

Every endpoint group ships with a seeder, test data, and API tests.

## MCP tools — the AI-review surface

This is what makes the differentiator real. Claude, in a session, reads the actual history and helps plan. Read tools are as important as writes:

- **Movements/workouts/progressions**: `list_/get_/search_/create_/update_/delete_` for each; `add_movement_to_workout`, `reorder_workout`, `link_related_movement`.
- **Sessions**: `log_workout_session`, `get_session`, `list_sessions` (date range), `complete_session_movement`.
- **Stats**: `record_measurement`, `list_measurements` (metric + range), `get_metric_trend`.
- **Log**: `add_fitness_log_entry`, `list_fitness_log` (date range, tag), `get_fitness_log_entry`.
- **The capstone**: `review_fitness_progress` (pulls recent sessions + measurements + log into one structured payload) and `draft_progression` (Claude proposes the next cycle from that history for review before save). The recipes analog is `ai_import_from_url` — the AI verb that defines the feature.

## Frontend (Vue) — top nav

```text
Workouts  |  Movements  |  Progressions  |  Log  |  Stats
```

Views: `MovementsView` (filterable library, favorites), `MovementDetailView` (how-to markdown, alternates/antagonists, "in these workouts"), `WorkoutsView`, `WorkoutDetailView` (ordered movements, "start session" button), `ActiveSessionView` (the mobile-critical logging screen), `ProgressionsView` + detail (week/phase timeline), `FitnessLogView` (dated markdown entries), `StatsView` (shared stats kit). Each `AddEdit*Modal` wrapper per the per-entity convention. Markdown via a shared `markdown-it` util.

## Stats page

Shared stats kit (`StatsSummaryCards`, `StatsTable`, `useStatsCharts`, theme-driven colors). Candidate charts: strength metrics over time (line, per lift), 5k time trend, mobility ROM trend (knee-to-wall, toe reach), session frequency by week, movements-by-kind mix, favorites coverage. All colors via theme CSS custom properties — no hardcoding.

## Migration & Seed

The `~/shart/fitness/` content becomes the initial catalog, so the app isn't empty on first boot:

- **Movements**: the exercises across `ppl/*.md`, `general-workouts/apartment-*.md`, `michael-workout-*.md`, `flexibility-moves.md`, plus the movements named in the three research plans → `movements` rows with tags/muscles.
- **Workouts**: each `ppl/*.md` sheet, each apartment/michael workout, and the plans' named sessions (A/B/C/D) → `workouts` with ordered `workout_movements` carrying the transcribed sets/reps/load.
- **Progressions**: `lower-body-plan.md` (12-week run return) and `shoulder-plan.md` (rehab arc) → `progressions` sequencing their sessions; `dance-conditioning.md` shelved as a `paused` progression.
- **Goals/metrics**: seed `metric_definitions` from `shart/fitness/goals.md` (deadlift working weight, 5k time, knee-to-wall L/R, toe reach, bodyweight) and `program.md`'s Week-0 baseline test as the first `measurements` once recorded.

Mechanism: a one-time seeder or an MCP-driven import pass (Claude reads the markdown and calls the create tools) — decide during Phase 1. Prefer the MCP import pass since it exercises the tools against real content, exactly like the recipes backfill validated that model before automation.

## Phased plan

Each phase independently shippable.

- **Phase 0 — Scaffold.** Repo → running app: FastAPI skeleton, Postgres + Alembic, Vue shell, Docker/Traefik, MCP server stub, CI. Nothing domain-specific; just the walking skeleton the phases below land in. *Acceptance*: `hello` endpoint + empty Vue shell reachable through Traefik, one MCP tool responds.
- **Phase 1 — Movements core.** `Movement` + kind/relationship lookups, CRUD, search/filter, favorites, MCP read+write, Vue library + detail + modal, **mobile-first patterns established here**, seeder. Import shart movements. *Acceptance*: browse/search/favorite a unified library of exercises, stretches, and poses on a phone.
- **Phase 2 — Workouts + relationships.** `Workout` + `WorkoutMovement` ordered composition, `movement_relationships` self-ref + swap-alternate UI, MCP. Import shart workouts. *Acceptance*: build/compose a themed workout, swap a movement for an alternate.
- **Phase 3 — Sessions (logging).** `WorkoutSession` + `SessionMovement`, start-from-template, checkboxes, actuals, per-session notes. The `ActiveSessionView`. *Acceptance*: do a workout on the phone, check off sets, record real weights, add a note.
- **Phase 4 — Measurements + Stats.** Metric definitions, measurement logging, `StatsView` with the shared kit. *Acceptance*: log a lift/ROM/time and see the trend chart.
- **Phase 5 — Fitness log.** Dated journal entries, MCP read tools. *Acceptance*: write and browse journal entries; Claude can read them.
- **Phase 6 — Progressions + AI drafting.** `Progression` sequencing, `review_fitness_progress` + `draft_progression` MCP capstone. Import the research plans as progressions. *Acceptance*: an active progression drives "what's next," and Claude drafts the following cycle from real session/stat/log history.

## Open questions (before/while building)

1. **Entity name: "Progression" vs. "Cycle"/"Mesocycle"?** The app is *meso*; the table is currently `Progression`. `Cycle`/`Mesocycle` would echo the app name and lifting vernacular. Pick one — it names a table, routes, and MCP tools.
2. **Unified `Movement` entity — agree?** Recommended (one entity + kind). Only reason to split is if poses/stretches diverge far more than expected.
3. **Movement relationships now or later?** Recommended in Phase 2 (self-ref, because swap-alternate is a core goal). Defer-to-tags is the conservative alternative.
4. **Muscle/region tagging — lookup table or free tags?** Lookup gives clean filtering and a body-map UI later; tags are lighter. Leaning lookup, since "show me posterior-chain movements" is a real filter.
5. **Offline session logging (PWA)?** Gym wifi is unreliable. UUID7 PKs keep offline-create possible, but full offline sync is a large add — worth it, or is a good responsive web app enough for v1?
6. **Reuse ichrisbirch's Authelia/JWT auth, or a simpler single-user gate?** Standalone means auth is now its own decision. Single-user self-hosted may only need a thin gate; decide in Phase 0.
7. **Stays private-only** — assumed yes. No multi-tenant scope.

## Out of scope (v1)

- Wearable/health-app import (Apple Health, Strava) — later.
- Auto-deriving lift stats from session data — measurements are manual in v1; the `source` field leaves the door open.
- Video attachments for movements — `source_url` link only.
- Social/sharing — private, like the rest of the ecosystem.

## References

- Source content to migrate: `~/shart/fitness/` — `ppl/*.md`, `general-workouts/apartment-*.md`, `michael-workout-*.md`, `simple-machine-workout.md`, `flexibility-moves.md`, `deadpool-2-ab-workout.md`, `ryan-reynolds-ripped.md`, and the three research plans `shoulder-plan.md`, `lower-body-plan.md`, `dance-conditioning.md`.
- Goals & rotation the app formalizes: `~/shart/fitness/goals.md`, `~/shart/fitness/program.md`
- Structural precedents worth borrowing from ichrisbirch: the recipes URL-ingestion AI verb (`ai_import_from_url`), the shared stats kit, the per-entity AddEdit-modal convention, and the M2M-with-payload-vs-prose relationship test.
