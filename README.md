# meso

meso is a self-hosted, single-user, mobile-first training app. A library of movements composes into
workouts, and workouts sequence into goal-directed cycles. Sessions record each workout as it was
actually performed. Measurements track the numbers that show progress. A journal holds the rest.

The boundary is the reasoning. meso stores the training record and serves it. Drafting the next
cycle happens outside it, in a conversation with Claude. Claude reads `meso review --json` and
writes the cycle back with ordinary CLI commands. There is no server-side AI and no MCP server. The
`meso` CLI is the agent's surface.

The name is the thesis. A *mesocycle* is the unit of training planning: a multi-week block aimed at
a target. The value is that planning, unified across strength, running, mobility and dance. The
block entity is named `Cycle`.

meso depends on no other product. It has its own database, its own web app and its own CLI. Three
subsystems share the repo: the Go API in `api/`, the Go CLI in `cli/`, and the Vue SPA in `web/`.
Their layering and contributor conventions are in [CLAUDE.md](CLAUDE.md).

## Install

The CLI ships as a release archive for linux and darwin, on amd64 and arm64.

```bash
gh release download --repo datapointchris/meso --pattern 'meso_*_linux_amd64.tar.gz'
tar -xzf meso_*_linux_amd64.tar.gz meso   # the archive holds the one binary
install -m 755 meso ~/.local/bin/meso
meso update                               # every later release, verified against its checksums

task cli:install                          # or a dev build from a checkout, into ~/.local/bin
```

A dev build reports its `git describe` version and refuses `meso update`. Only a release replaces
itself. `go install github.com/datapointchris/meso/cli@latest` builds a binary named `cli` with
the same refusal, so install from a release.

The server is an API image and a web image, `ghcr.io/datapointchris/meso-api` and
`ghcr.io/datapointchris/meso-web`. `docker-compose.yml` runs them beside Postgres. Running it for
real is under Deployment, below.

## Starting from scratch

A local stack needs Docker and nothing else.

```bash
git clone https://github.com/datapointchris/meso && cd meso
docker compose -f docker-compose.dev.yml up --build -d                          # Postgres, the API and the SPA, built from source
docker compose -f docker-compose.dev.yml run --rm --entrypoint ./meso-seed api  # the lookup rows every write needs; safe to re-run
curl localhost:8088/health                                                      # {"status":"ok"}: migrated, and Postgres answered
```

Then browse to `http://localhost:8080`. Caddy serves the SPA there and proxies `/api/*` to the API
on 8088. Postgres listens on 5459. The API runs with `ENV=development`, which checks no credential
and treats every request as the admin.

The seed is its own step because migrations carry no content. It inserts the lookup vocabularies
and the muscle catalog, upserting each row, so a second run changes nothing. Everything else
arrives through the CLI or the app.

To work on one piece from source, stop its container and run it with `task`. `task api:dev` runs
the API against the compose Postgres and migrates on start. `task web:dev` runs Vite on 3001 and
proxies `/api` to 8088. `task setup` installs web's npm dependencies first, which a fresh checkout
lacks. `task test` runs every suite. The API's handler tests start a throwaway Postgres through
testcontainers, so they need Docker, or `TEST_DATABASE_URL` naming a database to reuse.

The CLI needs a login before any command that reaches the API. That holds against every API,
including the development one, which ignores the token.

```bash
meso auth login https://meso.example.com  # fills in the config file, then prints a code to approve in any browser
meso auth status                          # api, issuer and client_id, and where each came from; exit 1 if any is missing
meso movements list                       # proves the token reaches the API
```

`auth login` fills in only what the config file lacks. It prints each value with its source before
anything is sent. It never rewrites a value the file already holds. A URL naming a different API
from the file's is refused, naming the file to edit. The file is written only once the login
succeeds.

## The grammar

The noun comes first and the verb last: `meso workouts list`, then `meso workouts show 3`. Every
top-level noun is a training noun. Commands about the software rather than the training live under
`meso admin`. Every list and show takes `--json`. Any partial command run with no arguments prints
what comes next, so the tree is walkable without this file.

The root help groups the nouns by what they hold:

- **Planning** — `movements`, `workouts`, `cycles`. The library and what is composed from it.
- **Training record** — `sessions`, `log`, `measurements`, `metrics`. What happened, and the
  vocabulary it is measured in.
- **Review** — `stats`, `review`. Reads across everything above.
- **App** — `admin`, `auth`, `config`, `update`, `completion`. The software, not the training.

A name is accepted where a command reads and never where it writes. `show` on movements, workouts
and cycles takes an id or a name. `edit`, `delete` and the composition verbs take ids. A name
that narrows to the wrong record costs a wrong screen on a read and a wrong row on a write. A name
matching several records prints one ready-to-run command per match, and `--json` returns the same
candidates as data.

Exit status is 0 on success, 1 on a runtime failure and 2 on a usage mistake. `--no-input` never
prompts. It fails instead, naming the flag that would have answered.

## Which surface answers which question

| Ask | Surface | Reads |
| --- | --- | --- |
| What do I train today | the web app's Sessions tab | the unfinished session, then the workouts to start one from |
| Log a set mid-workout | the web logging screen, `meso sessions set add` | the entry's last set, or its target |
| How is this movement done | the Movements tab, `meso movements show` | the movement's how-to, form cues and common faults |
| Build or change a workout or cycle | the Workouts tab, `meso workouts`, `meso cycles` | the library and its compositions |
| Is a number going up | the Stats tab, `meso measurements trend`, `meso stats` | measurements against their metric definitions |
| What should the next block be | `meso review --json`, read in a conversation | active cycles, recent sessions, measurements and journal entries |
| What papercuts were captured | `meso admin feedback list` | feedback rows, for the admin group only |
| Where is the CLI pointed | `meso auth status`, `meso config show` | the config file and the token store |
| Script against the data | the REST API under `/api/v1/` | every resource |

## Defined by what it leaves out — every feature is measured against the exclusions

meso is a tool for writing down movements and making planned, informed training blocks. It applies
the programming a bodybuilder brings to a mesocycle, or a 5k coach to a periodized race build. It
unifies strength, running, yoga, stretching and dance. It holds several concurrent goals at once.
The intelligence is in the planning, not in tracking for its own sake.

These are hard exclusions:

- No instructional videos, GIFs or animations. A movement carries a text how-to and form cues.
- No motivational content, quotes, coaching voice or "you got this" copy.
- No gamification: no streaks, badges, points, levels, confetti or achievement popups.
- No social feed, sharing, followers or leaderboards.
- No calorie, macro or nutrition tracking.
- No ads, upsell, paywalled basics or "AI coach" mascot. The AI here is a reasoning surface over
  real data, reached through the CLI.
- No forced onboarding, notifications by default or engagement mechanics.
- No rest timer and no plate calculator, though the market rewards both. The session model below
  says why.

Being single-user and self-hosted is what makes this discipline free. No growth metric pushes bloat
in. When a feature is in doubt, the default is to leave it out.

**Multiple concurrent goals are first-class.** Strength, a slow 5k build, mobility and dance
conditioning progress in parallel. Each has its own cycle and target metric. They share one
movement library and one weekly reality. The app holds several goals at once without making
anyone pick one.

**The wedge is the gap every incumbent shares.** The survey covered strength loggers (Strong, Hevy,
FitNotes, Jefit, Boostcamp), running planners (Couch-to-5k, Runna, TrainingPeaks, Garmin Coach, Nike
Run Club), yoga and mobility apps (Down Dog, Yoga Studio, Alo Moves) and hybrids (Fitbod, Strava,
Centr). None unifies strength, endurance and mobility. Every one assumes a single active plan.
Hybrid athletes stitch two specialist apps together and become the integration layer themselves:
"running apps don't know you lifted heavy yesterday." The mesocycle-programming niche is crowded
(meso.fitness, MesoBuilder, MesoTrack, Mesostrength). Every one of those is single-modality
strength, and closed.

**Text-first movement libraries are underserved.** The best reference content is text, from ExRx,
Muscle & Strength and StrengthLog. It gives numbered steps, a fault-to-fix list of mistakes and
honest primary and secondary muscle tagging. Most apps bury it in video.

**What users reward shaped the model.** The previous session's numbers sit inline on the logging
screen. A cycle's intensity is an effort target ("easy / Zone 2"), not pace alone, which answers
the universal "easy runs aren't easy" complaint. A cycle is legible: its phases and conditions say
why the next block is what it is. Swapping a movement carries its target to the substitute. The
movement library exports as CSV, through `meso movements export`.

**What users uninstall for confirms the exclusions.** The list is social feeds, gamified streaks,
video dependency, chatty motivational voiceovers, opaque "AI coach" black boxes, paywalled basics,
forced onboarding, calorie creep, and class libraries that answer "watch what?" instead of "do what
today?"

## The model — a plan and its performance are different rows

A folder of markdown plans works as storage but not as a system. It cannot answer what to do today.
It cannot record that it was done, or show a lift rising over three months. Several plans each
define their own week, and nothing reconciles them into one rotation. Two things make meso worth
building as an app:

1. **Template versus instance.** A workout is a plan. Doing it on a Tuesday is an event, with
   actual weights, checkboxes and how it felt. The instance data is the point of tracking.
2. **Drafting the next cycle from history.** Sessions, measurements and the journal are all
   reachable through the CLI. Claude reads what got done, what stalled and how it felt, and helps
   write the next cycle.

```text
Movement ──< WorkoutMovement >── Workout ──< CycleWorkout >── Cycle
   ↺                                │                           │
self-join: alternate,               │ a session may             │ target_metric
antagonist, progression,            │ start from one            ▼
regression, see_also                ▼                    MetricDefinition ──< Measurement
                             WorkoutSession ──< SessionMovement ──< SessionSet
                             (on a date)        (the plan)          (what was performed)

FitnessLogEntry   the dated journal a review reads
Feedback          papercuts about the app itself
```

The schema itself is `api/migrations/`.

### Movement — exercises, stretches and poses are one entity

Exercises, stretches and yoga poses are the same concept. So there is one `Movement` entity with a
`movement_kind` lookup (`exercise`, `stretch`, `yoga_pose`), not three near-identical tables.
Kind-specific fields are nullable and used where they apply. A pose has a Sanskrit name and a hold.
A lift has a default rep scheme. Favorites, tags, search and relationships stay uniform across all
three. A workout can mix a squat, a couch stretch and pigeon pose without crossing an entity
boundary.

- `name` (unique, the natural key), `movement_kind`, `load_mode`, `favorite`, `rating` (1–5,
  nullable)
- `load_mode` — `weighted`, `bodyweight`, `timed` or `assisted`. It decides what the logging screen
  asks for. A weight box on an ab exercise is a question with no answer. `movement_kind` cannot
  stand in, since a back squat and a nordic curl are both exercises.
- `tags: TEXT[]` — the "good for" dimension (mobility, posterior-chain, desk-counter,
  anti-rotation)
- muscles are lookup-backed, not free tags. A `muscles(name PK, region)` lookup feeds a
  `movement_muscles(movement_id, muscle, role)` join, where role is `primary` or `secondary`. That
  is the honest primary/secondary tagging ExRx uses. `region` drives filtering ("show me
  posterior-chain movements") and leaves room for a body-map UI.
- `equipment: TEXT[]` — barbell, dumbbell, mat, none
- `how_to` (markdown) — how to perform it, step by step, or a liked sequence written out
- `form_cues` (markdown) — what to watch for to keep good form
- `common_faults` (markdown) — the usual failures and mistakes
- kind-specific nullable fields: `default_sets`, `default_reps`, `default_hold_seconds`,
  `sanskrit_name`
- `measurable_rom` — flags a movement whose range of motion is worth logging as a measurement
  (knee-to-wall)
- `source_url`, `source_name` — where it came from, a plan or a video
- `created_at`, `updated_at`

**The how-to fields are text, deliberately.** The need is "I forget exactly what a prisoner squat
is, how to do it, what to watch for, and what commonly goes wrong." A movement carries enough
writing to reconstruct it, or to explain a sequence built from it, and nothing more. A
prisoner-squat entry reads: *how-to* — hands laced behind the head, elbows wide, squat to depth
keeping the chest up; *form cues* — elbows stay back, knees track over toes, weight mid-foot;
*common faults* — elbows collapse forward, chest drops, heels lift. That is the whole ambition of
the field.

**Author them as markdown, not as one run-on paragraph.** They are read on a phone, mid-set. A
how-to that packs execution, tempo and progression into one block reads as a wall no typography can
rescue. The seam is almost always the same: what to do, then why it works or how it differs from
the movement next to it. That is two paragraphs. Where the prose enumerates a protocol (contract,
relax, take up slack; a balance progression), make it a list.

### Relationships — a self-join earns its table from the swap

`movement_relationships(movement_id, related_movement_id, relationship_kind)` is a directional
self-join. Its kind is `alternate`, `antagonist`, `progression`, `regression` or `see_also`. A
movement cannot relate to itself. `progression` and `regression` are the strength terms for a
harder or easier variant of a movement, unrelated to `Cycle`.

A relationship deserves a table only when a genuine query sits behind it. Here one does. Swapping a
movement for a related one while building a workout is a concrete UI action, and the workout
builder offers the movement's relationships as its substitutes. Antagonist, progression and
regression ride the same table for free.

### Workout — an ordered composition, read on every render

A workout is its ordered list of prescribed movements. The join carries a payload and drives the
render, so a many-to-many with a payload is correct here rather than over-engineering.

- `Workout`: `name` (unique), `theme` (nullable — "push", "lower + shoulder rehab"),
  `tags: TEXT[]`, `notes` (markdown), `favorite`, `estimated_minutes`, `created_at`, `updated_at`
- `WorkoutMovement`, the ordered join: `workout_id`, `movement_id`, `position`, `sets`,
  `reps: TEXT` ("4–6", "AMRAP", "30s"), `load: TEXT` ("80% 1RM", "2 plates", "bodyweight"),
  `rest_seconds`, `superset_group` (nullable), `notes`

Each entry has a surrogate id and a deferrable `UNIQUE(workout_id, position)`. A reorder swaps
positions inside one transaction without tripping the constraint. A movement a workout uses cannot
be deleted from under it, and the API answers 409.

### Session — a workout performed on a date, kept apart from its template

A session is the record of what was actually done. Keeping it distinct from the template is what
makes history real data.

- `WorkoutSession`: `id` (UUID7), `workout_id` (nullable — a free-form session has none),
  `performed_on`, `duration_minutes`, `overall_notes` (markdown), `felt` (nullable energy or mood
  tag), `created_at`, `finished_at` (nullable)
- `SessionMovement`, the plan for this session: `session_id`, `movement_id`, `position`, `done`,
  `target_sets`, `target_reps`, `target_load`, `notes`
- `SessionSet`, what was performed, one row per set: `session_movement_id`, `position`,
  `reps: INT`, `load: TEXT`, `hold_seconds`, `set_kind` (`working`, `warmup`, `amrap`, `drop`,
  `failure`), `notes`, `logged_at`

Deleting a workout keeps its sessions and sets their `workout_id` to null. The logged history of
having done it survives as a free-form record.

**The plan and the performance are different columns, and that is the point.** `target_*` is
copied from the workout when the session starts and then left alone. The sets are the record.
Training that went differently from the program stays a fact rather than reading as a failure.
Doing four sets of a programmed three is four rows, not an overwrite.

A set carries its own `set_kind` and `hold_seconds`, so its `reps` can be a plain integer. "AMRAP"
is a kind, and a 30-second hold is a hold. `load` stays free text, matching the prescription it is
measured against.

**Logging a set costs one tap.** `POST .../sets` with an empty body repeats the previous set, with
the same reps and load. The first set falls back to the target. Only a set that differs needs
describing. `done` ticks itself the moment the logged sets reach the target, and an entry with no
target is done on its first set. `done` stays writable, so stopping short on purpose survives the
sets that follow.

**A session with no template is the unplanned path, and it is where workouts come from.** Not
everything is programmed in advance. Requiring a workout before anything is recorded means
authoring a template mid-gym or not logging at all. So a session can start empty and gain movements
as they happen. It can then be promoted into a `Workout`. The prescription becomes as many sets as
were logged, the reps most of them shared, and the load of the last set. The session is
back-linked to the workout it produced, so it reads as that template's first instance. Only a
free-form session can be promoted. Promoting one already backed by a template would silently fork
it, which is an edit of that workout rather than a second one.

**Any session can gain or lose movements, template-backed or not.** Doing something the plan did
not call for, or skipping something it did, is part of what happened. Editing the workout instead
would rewrite the plan of every session ever run from it.

**`finished_at` separates training happening now from history.** Finishing fills in
`duration_minutes` from when the session started, so the duration is never typed. It is also what
lets the app offer to resume. Finishing twice keeps the first timestamp. Finishing means "I am done
training", not "I completed the plan". Nothing compares it to the targets. A session finished two
movements in is a finished session.

Session detail embeds each entry's `previous`: the last set of the most recent performed entry for
that movement, strictly before this session. An entry qualifies only if it is done and has sets
under it. Ticked with nothing logged is not a number to beat. The list endpoint leaves `previous`
null. One `DISTINCT ON` query resolves it for the whole session.

A swap re-points a session's entry to another movement, and its target and logged sets carry over
to the substitute. The API and `meso sessions movement edit --movement` offer it. The web
logging screen does not.

**The rest timer and the plate calculator are deliberately excluded.** The rest timer is not wanted.
The plate calculator is structurally wrong for this library. `load` is free text by design
("80% 1RM", "2 plates", "bodyweight"), so there is no number to calculate from without parsing
prose.

### Cycle — workouts sequenced toward a target, advanced by readiness

A research-backed plan, such as a 12-week run return or a shoulder-rehab arc, is a cycle. Its
workouts flow over weeks toward a target. `Cycle` names the table, the routes and the CLI commands.

- `Cycle`: `name` (unique), `goal_summary`, `target_metric` (nullable, a metric definition such as
  "deadlift-working-weight"), `target_value`, `target_date` (nullable — for a race-anchored build),
  `start_date` (nullable), `status` (`planned`, `active`, `paused`, `completed`), `notes`
- `CycleWorkout`, the ordered join: `cycle_id`, `workout_id`, `position`, `week` (nullable),
  `phase` (nullable — "base", "build", "taper"), `frequency` ("2×/week"), `intensity` (nullable —
  an effort or heart-rate target, "easy / Zone 2"), `conditions` (nullable — "when knee-to-wall
  symmetric, advance")

On read, each `CycleWorkout` also carries `last_performed_on`. It is the newest day a session of
that workout had a movement done, or null. It is derived from sessions and never stored. A session
started from the workout and abandoned with nothing done does not count.

Changing or clearing a cycle's `target_metric` without supplying a value clears the old
`target_value`. A target never outlives the metric it was measured against.

A cycle is humane rather than a straitjacket, which answers the top structural complaint about
running apps:

- **Nothing is ever marked failed.** A cycle holds no calendar of due dates, so a missed day is not
  recorded as anything. It advances by readiness, not by weekday.
- **A week repeats until ready.** This is Couch-to-5k's most-loved safety valve. Each entry's
  `conditions` says what earns the next block, and nothing advances a cycle on its own.
- **Legible, not opaque.** `phase`, `conditions` and the target explain why the next block is what
  it is. People distrust black-box adaptive plans. Legibility is also what makes a drafted cycle
  reviewable rather than magic.

### Measurement — a metric carries its own protocol

Lift numbers, 5k time, toe reach, bodyweight, range of motion: a metric definition, and a time
series of values against it.

- `metric_definitions`: `name` (the key — "deadlift-working-weight", "5k-time",
  "knee-to-wall-left"), `label` (the display string, defaulting to the name title-cased), `unit`
  ("lb", "seconds", "cm"), `direction` (`higher_better` or `lower_better`), `category`
  (`strength`, `cardio`, `mobility`, `body`), `how_to_measure` (markdown — the protocol that
  produces the number)
- `measurements`: `metric`, `value: NUMERIC`, `measured_on`, `source` (`manual` or `session`),
  `notes`

**A metric carries its own protocol.** Unit, direction and category are what a chart needs. None of
them says what the thing is. "Heel Raise Capacity Right" names a number without naming how to get
one. It matches no movement in the library, so there is nowhere else to look it up. A series is
only a series if every reading was taken the same way. An undocumented metric silently degrades
into readings that are not comparable. One field covers it, not a description plus a how-to,
because for every metric those are the same sentence.

The name is the slug the API, the CLI and cycle targets all address a metric by. That makes it
permanent, and a poor thing to show a human. `label` is edited independently, so the app never
displays a raw key. Renaming a metric is a delete and a redefine. A metric with recorded readings
cannot be deleted.

The stats payload carries every defined metric, measured or not. An unmeasured one comes back with
no points and renders as an empty card. The page shows what can be tracked, not only what already
has history.

### Journal — the entries a review reads

- `FitnessLogEntry`: `id` (UUID7), `entry_date`, `body` (markdown), `tags: TEXT[]`, `mood`
  (nullable), `created_at`, `updated_at`

The journal sits beside the notes on any one workout. It is the record of the whole journey, and
the substrate `meso review` hands to Claude.

### Feedback — owned outright and forwarded nowhere

The one entity here that is not training data. A button on every screen captures a thought
mid-session, at the gym, one-handed, so it survives the moment it was had. Triage happens in the
CLI, under `meso admin feedback`.

- `Feedback`: `id` (UUID7), `status` (`open` or `done`), `body`, `context_path` (the in-app route
  it was raised from), `viewport_width` and `viewport_height` (the window it was seen at, null for
  a CLI capture), `created_at`, `updated_at`

**meso owns this outright and forwards it nowhere.** No row refers to another system. No other
system needs to be reachable for a capture to succeed. Whoever wants the data comes and reads it,
with `meso admin feedback list --json`. The dependency points inward, at meso, never sideways from
meso at something else. That is what keeps the app standalone: it runs unchanged wherever it is
copied.

There is deliberately no kind or category, such as bug, idea or improvement. It costs a tap at
capture time and changes nothing at read time. The body says what it is, and the response is the
same either way. The app's exclusion list applies to its own internals.

### Storage — lookup tables, TEXT columns, and two kinds of key

- Every text column is `TEXT`, never `varchar(n)`.
- A categorical is a lookup table with a `TEXT PRIMARY KEY` and a foreign key, never an enum. The
  lookups are `movement_kinds`, `relationship_kinds`, `cycle_statuses`, `set_kinds`, `load_modes`
  and `muscles`. A small bounded attribute of one table uses a `CHECK` instead: a muscle's role, a
  metric's direction and category, a measurement's source, a feedback status.
- A user-generated row keys on a UUID7 minted in Go: sessions, journal entries and feedback. A
  catalog row keys on a Postgres identity and keeps its `name` unique: movements, workouts and
  cycles. UUID7 keeps offline creation possible.
- Filtering is server-side. A list endpoint owns its query parameters and builds the `WHERE` in
  SQL, so the CLI and the web share one filter definition.
- Migrations are goose DDL, applied when the API starts. No content rides them.

**The seed carries only the foreign-key backbone.** `cmd/seed` writes the lookups no CLI verb
writes, and the muscle catalog. It runs after every deploy, idempotently, so no fresh environment
comes up unable to write. Everything else is content loaded through the CLI (`meso metrics
define`, `meso movements create`, `meso workouts create`, `meso cycles create`). That exercises the
real API write path. The rule: an entity with a CLI verb is imported, never seeded.

## The web app — the screen opened one-handed between sets

This is an iOS-in-the-gym app, and mobile-first is the whole point rather than a later pass. The
SPA is Vue 3 and TypeScript on Vite, with `<script setup>` components, Vue Router and SCSS. It has
no login page. A read the API answers with 401 sends the browser to the API's login route, which
logs it in through the identity provider and brings it back to the page it was on. A write it
answers with 401 fails instead and says the change was not saved, so nothing is lost silently.

- The minimum interactive target is 44px, the `--touch-target` design variable. The logging screen
  works one-handed without zoom.
- Layouts are responsive from a phone viewport up, with no horizontal scroll.
- **Sessions is the first tab and the landing route.** The tabs are Sessions, Workouts, Movements,
  Log and Stats. They render as a top bar on a desktop and a bottom tab bar on a phone. The reason
  to open the app is to train, so the first screen offers to resume the unfinished session or start
  one. Cycles gave up its tab. Planning a block is a rare, considered act, reached from the Workouts
  header.
- **The logging screen is the highest-traffic screen.** Each movement has one large "Log set"
  button. It sends an empty body, and the server repeats the last set. The plan sits above it as a
  line of text, not a row of inputs. The last performance shows beside it as "Last: …". Nothing on
  the screen scores you: "3 movements · 20 sets", not "2/5 done".
- **Nothing irreversible happens without a dialog, and never through `window.confirm`.** A system
  alert lands its buttons wherever iOS decides. Its "OK" says nothing about what is being agreed
  to. That is the wrong shape for a destructive action taken one-handed mid-workout. Every
  confirmation goes through `useConfirm`, which renders one in-app dialog.
- **The movement picker and the date field are in-app components.** A native `<select>` or
  `<input type="date">` hands off to an OS picker whose density is fixed and small. Either costs
  several interactions where one would do. The movement picker is a search box over tap targets,
  used by the workout builder and the logging screen. The edit modals, the cycle screens and the
  movement page's relationship form still use a native `<select>`.
- A feedback button sits on every screen. It records the route and the viewport it was raised at.
- The theme is a CSS-variable switch on `data-theme`, dark or light.

The stats page charts each defined metric's trend, grouped by category. Beside them sit session
frequency by week, the library's mix by kind, and counts of movements, favorites and sessions. Every
color comes from a theme custom property.

## The CLI — the agent's surface, and the power user's

`meso` is a thin REST client over the API, written in Go with cobra. It shares the API's REST
contract rather than anything with the frontend, which is why it is Go and not TypeScript. Claude
drives it through a shell and reads its `--json`.

```bash
meso workouts show "push day"                  # by name on a read; writes take the id it prints
meso sessions log --from-workout 1             # start a session with the workout's plan as its target
meso sessions set add <session-id> <entry-id>  # one more set like the last
meso sessions finish <session-id>              # end it; the duration fills itself in
meso review --since 12w --json                 # active cycles and recent history, in one payload
```

**The AI capstone is a read plus ordinary writes, with no server-side model.** `meso review --json`
pulls the active cycles and the recent sessions, measurements and journal entries into one
structured payload. Claude reads it and reasons about the next block in the conversation. It
persists the draft with ordinary writes, `meso cycles create` and `meso cycles workouts add`. The
CLI needs only solid read and write primitives. Claude is the reasoning.

**Login is the OAuth 2.0 device authorization grant (RFC 8628).** `meso auth login` authenticates
as a public client with the scopes `openid`, `profile`, `offline_access` and `groups`. The CLI
prints a code and a URL, and approval happens in any browser on any device. That is what makes
login work over SSH, where a loopback redirect would need a browser able to reach a listener on
the machine being logged into. The access token is an RFC 9068 JWT (`typ=at+jwt`), which the API
verifies itself. The token is kept in the OS keychain, or in a file only this user can read on
a host with no keychain. `meso auth status` says which. Clients are registered per machine and app,
`meso-cli-<short hostname>` by default, so revocation has both axes. `meso auth token` prints the
token for a `curl -H "Authorization: Bearer $(meso auth token)"`.

**Where the CLI is pointed is one file**, `$XDG_CONFIG_HOME/meso/config.toml`, which defaults to
`~/.config/meso/config.toml`. It holds `api`, `issuer` and `client_id`. Every command reads it.
`--api` is the only thing that outranks it. There is no environment variable and no compiled-in
address. `meso auth login <url>` fills in only what the file lacks: `api` from the URL, `issuer`
from the API's RFC 9728 metadata at `<url>/.well-known/oauth-protected-resource`, and `client_id`
from `--client-id` or the hostname default. `meso config show` prints each setting and where it
came from. The file and its rules are [goclilogin's](https://github.com/datapointchris/goclilogin).

An `--api` that differs from the file's is checked before any token is sent. Its RFC 9728 metadata
must name the file's issuer. So a CLI pointed at a local API with `--api http://localhost:8088`
needs the API run with `PUBLIC_URL=http://localhost:8088`, as `docker-compose.dev.yml` does, and
with an `OIDC_ISSUER` matching the file's issuer.

**`admin` is the namespace for operating the app, as distinct from using it.** Every other
top-level command is a training noun, which is what lets `meso --help` read as a description of
the domain. Commands about the software itself go under `admin` instead of diluting that. The
convention is HashiCorp's (`vault operator`, `consul operator`). It is named
`admin` because "operator" there means cluster and consensus lifecycle, and meso has no cluster.

**meso has one user, and the identity provider it signs in through can hold more.** A login admits
a caller to every training route. Administration also needs the identity provider's `admin` group.
`meso admin` stays in the CLI's help for anyone who holds the binary. Reading, editing and deleting
feedback answer 403 to a non-member. Capturing it needs only a login. A browser's groups are the
ones the identity provider named when its session logged in. A CLI request carries them in the
access token's `groups` claim, which `meso auth login` requests.

**`update` sits at the root rather than under `admin`.** meso checks once a day for a newer release
and prints a single line: "meso vX available (running vY) — run `meso update`". The shared library
builds that sentence from the binary name, with no way to name a nested command. Under `admin` the
notice would point at a command that does not exist. The check never installs anything and never
prints an error. It skips a dev build, a non-terminal and CI. Set `NO_AUTO_UPDATE` or
`MESO_NO_AUTO_UPDATE` to turn it off.

## The API — one route table, filtered in SQL, behind three kinds of credential

The API is one Go binary on `net/http`'s ServeMux with method routing. It reaches Postgres through
pgx and applies goose migrations at startup.

```bash
curl localhost:8088/health                                    # unauthenticated; 503 when Postgres does not answer
curl -H "Authorization: Bearer $(meso auth token)" \
  "https://meso.example.com/api/v1/sessions?unfinished=true"  # the session underway, as the CLI sees it
```

`api/handlers/router.go` is the route table, and the only one. Production serves it, and the
handler tests build it against a throwaway Postgres, so the two cannot drift. A path under
`/api/v1/` that no route matches is a 404. A method its path does not take is a 405 naming the
methods it does take.

A resource flows through `models/` (wire structs), `repository/` (all SQL), and `handlers/` (HTTP,
mapping repository errors to status codes), one file per resource. A list endpoint owns its filters
as query parameters. A detail response embeds its relations, so one GET renders a screen. A workout
carries its ordered entries, a session its entries and their sets, a cycle its ordered workouts,
and a movement its muscles and related movements. The database owns computed values such as
timestamps, never the client. Every resource has a testcontainers-backed handler test.

Three kinds of credential reach it. A bearer credential's shape decides which check grades it,
before either check runs, so one cannot be graded by the other's rules.

- **A JWT bearer is the CLI.** The API checks, in order, that the header is `typ=at+jwt`, that the
  signature verifies against the issuer's JWKS, that `iss` matches, that `sub` is non-empty, that
  `client_id` starts with `meso-cli-`, and that `exp` is in the future. Every rejection is the same
  401 with the same message, so a probe cannot learn which check it failed. An identity provider
  that does not answer is a 503 instead, because logging in again cannot fix a network failure.
- **Any other bearer is an internal service-to-service caller.** It must equal
  `MESO_SERVICE_TOKEN`, compared in constant time. It names no user and carries no group.
- **No bearer is the browser.** The API is the identity provider's confidential client `meso-web`,
  and logs the browser in at `/api/auth/login`. The session lives in the API's database. The
  browser holds one `HttpOnly`, `SameSite=Strict` cookie naming it, and no token. Every request
  carrying that cookie must also send `X-CSRF: 1`, which a page on another origin cannot add. No
  header identifies anyone, so a request sent straight to the container proves nothing. A request
  carrying any bearer never reaches the session, so a junk token cannot be paired with a stolen
  cookie.

`client_id`, not `aud`, isolates meso from other products on the same identity provider. Authelia
does not carry the audience through the device grant, so the `aud` claim comes back empty and
cannot be relied on. `authelia.bearer.authz` is absent from the scopes because Authelia forbids the
device grant alongside it.

`/health`, `/.well-known/oauth-protected-resource` and the two login routes need no credential.
Health answers 503 when Postgres fails, so an orchestrator sees the outage. The metadata is what a
CLI reads before it holds a token, and the API serves it only while `PUBLIC_URL` is set.
`/api/auth/login` and `/api/auth/callback` are what a browser reaches before it holds a session.

With `ENV=development` none of this runs. Every request is accepted and placed in the admin group.

## Deployment — images are pulled, never built on the host

A push to `main` runs `.github/workflows/release.yml`. It validates first: lint, format, typecheck,
build and tests for web, and the Go jobs for api and cli. It then builds the api and web images and
pushes them to `ghcr.io/datapointchris/meso-api` and `ghcr.io/datapointchris/meso-web`, tagged
`:latest` and `:sha-<short>`. The `:sha-` tag never moves, so a rollback is a manifest edit. A push
with releasable commits under `cli/` also cuts a `cli/v*` release of the binaries.

When the workflow finishes, GitHub fires a repository `workflow_run` webhook. Its receiver lives
outside this repo. It pulls the checkout on the host and runs:

```bash
docker compose pull && docker compose up -d --no-build  # the images CI pushed; nothing compiles on the host
docker compose run --rm --entrypoint ./meso-seed api    # after up, once the API has migrated; idempotent
```

A webhook 200 means the deploy script started, not that it succeeded. The receiver's own log
answers "did it deploy?", and `gh run list` does not. The receiver deploys only when the finished
workflow's path is `release.yml`, so renaming that file stops every deploy. A manual workflow in
`.github/workflows/` re-fires the webhook, for a redeploy without a commit.

`docker-compose.yml` reads its secrets from env files that are never committed:
`/etc/meso/.env.postgres` for Postgres and `/etc/meso/.env.api` for the API. It sets `PUBLIC_URL`
inline, so a deployment edits it there. The API reads:

| Variable | What it sets |
| --- | --- |
| `DATABASE_URL` | the Postgres connection |
| `ENV` | `development` accepts every request unauthenticated; any other value enforces auth |
| `OIDC_ISSUER` | the identity provider whose JWKS signs the CLI's tokens, and which logs the browser in |
| `CLI_CLIENT_ID_PREFIX` | the `client_id` prefix a token must carry, `meso-cli-` by default |
| `WEB_CLIENT_ID` | the confidential client the API logs browsers in as, `meso-web` by default |
| `WEB_CLIENT_SECRET` | that client's secret; production refuses to start without it |
| `PUBLIC_URL` | the address the edge serves the API at; the metadata names it, a CLI refuses it unless it matches exactly, and the browser login's callback is under it. Production refuses to start without it |
| `MESO_SERVICE_TOKEN` | the shared secret for internal callers; empty disables that path |
| `PORT` | the listen port, 8088 by default |

**Set `ENV` in the env file.** Its default is `development`, which accepts every request
unauthenticated.

The edge in front of the web container has two duties:

- route the public hostname to the web container's port 80, behind the identity provider's
  ForwardAuth middleware;
- serve `/.well-known/oauth-protected-resource` without that middleware, since a CLI reads it
  before it holds a token.

The API reads no header the edge sets, so the edge is a second gate rather than the one that
identifies anyone.

The web container is Caddy. It serves the SPA with a fallback to `index.html`, and proxies
`/api/*` and the RFC 9728 path to the API. Both containers answer `/health` unauthenticated. The
API shuts down gracefully on SIGTERM and writes structured JSON logs to stdout.

## Out of scope

- Offline logging and sync, and an installable PWA. The SPA needs a connection and ships no web app
  manifest or service worker. UUID7 keys keep offline creation possible.
- Wearable and health-app import, such as Apple Health or Strava.
- Deriving lift stats from logged sessions. Measurements are entered by hand. `source` admits
  `session`, so a derived reading would need no schema change.
- Video attachments for movements. `source_url` links out instead.
- Social features and sharing.

## Why `meso` is a codebase name, not a brand

"meso" is accurate and spare for a private tool. It is a weak commercial mark. "Mesocycle" is an
industry term, so the name is descriptive. Several fitness apps already carry it on the same
periodization thesis: meso.fitness, joinmeso.com, MesoBuilder, MesoTrack and Mesostrength.
Registered marks such as MESO FIT and MESO METHOD sit in Class 41. None of that reaches a private,
self-hosted, single-user tool. A product name would be chosen separately if one were ever wanted,
and `meso` would stay the codebase's name.
