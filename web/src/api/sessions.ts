// Typed access to the sessions endpoints and their per-movement / per-set logging
// sub-resources. Types mirror the Go API's JSON wire contract; the DB owns id,
// timestamps, and the UUID7 session id (treated here as an opaque string).
import { http } from './client'
import type { LoadMode, MovementKind } from './movements'
import type { Workout } from './workouts'

// SetKind tags what a set was for. Everything is a working set unless it says otherwise,
// which is why the vocabulary costs nothing to ignore.
export type SetKind = 'working' | 'warmup' | 'amrap' | 'drop' | 'failure'

// SessionSet is one set as it was performed. Sets are the record of what happened; the
// entry's target_* is the plan they are measured against, and the two are deliberately
// not the same fields.
export interface SessionSet {
  id: number
  position: number
  reps: number | null
  load: string | null
  hold_seconds: number | null
  set_kind: SetKind
  notes: string
  logged_at: string
}

// PreviousActuals is the last performed result for a movement before this session —
// the number to beat, taken from the last set of it. Null when never performed, and only
// sent on session detail; the list endpoint leaves it null.
export interface PreviousActuals {
  performed_on: string
  sets: number
  reps: number | null
  load: string | null
}

// SessionMovement is one movement within a session: the target it was performed against
// and the sets actually performed. load_mode is what tells the logging screen whether to
// ask for a weight at all.
export interface SessionMovement {
  id: number
  movement_id: number
  movement_name: string
  movement_kind: MovementKind
  load_mode: LoadMode
  position: number
  done: boolean
  target_sets: number | null
  target_reps: string | null
  target_load: string | null
  sets: SessionSet[]
  previous: PreviousActuals | null
  notes: string
}

export interface Session {
  id: string
  workout_id: number | null
  workout_name: string | null
  performed_on: string
  duration_minutes: number | null
  overall_notes: string
  felt: string | null
  movements: SessionMovement[]
  created_at: string
  finished_at: string | null
}

// SessionCreate starts a session. workout_id copies that template's movements in as the
// target; omitting it starts a free-form session. performed_on defaults to today.
export interface SessionCreate {
  workout_id?: number
  performed_on?: string
  duration_minutes?: number | null
  overall_notes?: string
  felt?: string | null
}

// SessionUpdate is the partial update of session-level fields (the logged movements
// are managed through the sub-resource PATCH).
export interface SessionUpdate {
  performed_on?: string
  duration_minutes?: number | null
  overall_notes?: string
  felt?: string | null
}

// SessionMovementPatch overrides done / edits this session's target / swaps one entry.
// movement_id re-points the entry (the mid-session swap), carrying the target and the
// already-logged sets to the substitute.
export interface SessionMovementPatch {
  done?: boolean
  target_sets?: number | null
  target_reps?: string | null
  target_load?: string | null
  notes?: string
  movement_id?: number
}

// SessionMovementInput appends one movement to a session already underway. The target
// fields record an intention; what was performed is logged as sets.
export interface SessionMovementInput {
  movement_id: number
  done?: boolean
  target_sets?: number | null
  target_reps?: string | null
  target_load?: string | null
  notes?: string
}

// SessionSetInput logs one set. Every field is optional, and an empty body is the normal
// case: the server repeats the previous set, so logging one is a single tap.
export interface SessionSetInput {
  reps?: number | null
  load?: string | null
  hold_seconds?: number | null
  set_kind?: SetKind
  notes?: string
}

// SessionSetPatch fixes one logged set.
export interface SessionSetPatch {
  reps?: number | null
  load?: string | null
  hold_seconds?: number | null
  set_kind?: SetKind
  notes?: string
}

// SessionPromote turns what was performed free-form into a reusable workout: the logged
// sets become the prescription, so only the name and its labels are supplied here.
export interface SessionPromote {
  name: string
  theme?: string | null
  tags?: string[]
  notes?: string
  estimated_minutes?: number | null
}

export interface SessionFilter {
  workout_id?: number
  from?: string
  to?: string
  unfinished?: boolean
}

function queryString(filter: SessionFilter): string {
  const params = new URLSearchParams()
  if (filter.workout_id !== undefined) params.set('workout_id', String(filter.workout_id))
  if (filter.from) params.set('from', filter.from)
  if (filter.to) params.set('to', filter.to)
  if (filter.unfinished) params.set('unfinished', 'true')
  const q = params.toString()
  return q ? `?${q}` : ''
}

const base = (id: string) => `/sessions/${encodeURIComponent(id)}`
const setsBase = (id: string, entryId: number) => `${base(id)}/movements/${entryId}/sets`

export const sessionsApi = {
  list: (filter: SessionFilter = {}) => http.get<Session[]>(`/sessions${queryString(filter)}`),
  get: (id: string) => http.get<Session>(base(id)),
  create: (body: SessionCreate) => http.post<Session>('/sessions', body),
  update: (id: string, body: SessionUpdate) => http.put<Session>(base(id), body),
  remove: (id: string) => http.del(base(id)),
  // Finishing is idempotent, so a double tap cannot rewrite when training ended.
  finish: (id: string) => http.post<Session>(`${base(id)}/finish`, {}),
  // Per-movement composition — each write returns the refreshed session, so the logging
  // screen re-renders (including the new entry's previous actuals) from one response.
  addMovement: (id: string, body: SessionMovementInput) => http.post<Session>(`${base(id)}/movements`, body),
  updateMovement: (id: string, entryId: number, body: SessionMovementPatch) =>
    http.patch<Session>(`${base(id)}/movements/${entryId}`, body),
  removeMovement: (id: string, entryId: number) => http.delData<Session>(`${base(id)}/movements/${entryId}`),
  // Per-set logging. addSet with an empty body repeats the previous set, which is what
  // makes "Log set" one tap.
  addSet: (id: string, entryId: number, body: SessionSetInput = {}) => http.post<Session>(setsBase(id, entryId), body),
  updateSet: (id: string, entryId: number, setId: number, body: SessionSetPatch) =>
    http.patch<Session>(`${setsBase(id, entryId)}/${setId}`, body),
  removeSet: (id: string, entryId: number, setId: number) =>
    http.delData<Session>(`${setsBase(id, entryId)}/${setId}`),
  // Promotion returns the created workout, not the session.
  promote: (id: string, body: SessionPromote) => http.post<Workout>(`${base(id)}/workout`, body),
}

// setCount reports how many sets were logged across a whole session — what happened,
// rather than how much of the plan was hit.
export function setCount(session: Session): number {
  return session.movements.reduce((total, m) => total + m.sets.length, 0)
}

// isInProgress is the only thing about a session that changes what to do next.
export function isInProgress(session: Session): boolean {
  return session.finished_at === null
}

// targetSummary renders the plan an entry is measured against. Empty when there is none —
// a free-form entry is not measured against anything.
export function targetSummary(m: SessionMovement): string {
  const parts: string[] = []
  if (m.target_sets != null || m.target_reps) parts.push(`${m.target_sets ?? '?'} × ${m.target_reps ?? '?'}`)
  if (m.target_load) parts.push(m.target_load)
  return parts.join(' · ')
}

// previousSummary renders the last performed result as one line for the logging screen
// (e.g. "3 × 5 · 185lb · 2026-07-08"). Empty when the movement has never been performed,
// so the caller can skip the row entirely rather than render a placeholder.
export function previousSummary(m: SessionMovement): string {
  const p = m.previous
  if (!p) return ''
  const parts: string[] = [`${p.sets} × ${p.reps ?? '?'}`]
  if (p.load) parts.push(p.load)
  parts.push(p.performed_on)
  return parts.join(' · ')
}
