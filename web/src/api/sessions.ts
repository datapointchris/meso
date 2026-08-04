// Typed access to the sessions endpoints and their per-movement logging sub-resource.
// Types mirror the Go API's JSON wire contract; the DB owns id, timestamps, and the
// UUID7 session id (treated here as an opaque string).
import { http } from './client'
import type { MovementKind } from './movements'
import type { Workout } from './workouts'

// PreviousActuals is the last performed result for a movement before this session —
// the number to beat. Null when never performed, and only sent on session detail; the
// list endpoint leaves it null.
export interface PreviousActuals {
  performed_on: string
  actual_sets: number | null
  actual_reps: string | null
  actual_load: string | null
}

// SessionMovement is one logged entry — the done checkbox and the real (actual)
// numbers, with the movement's name/kind embedded for render.
export interface SessionMovement {
  id: number
  movement_id: number
  movement_name: string
  movement_kind: MovementKind
  position: number
  done: boolean
  actual_sets: number | null
  actual_reps: string | null
  actual_load: string | null
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
}

// SessionCreate starts a session. workout_id copies that template's movements in;
// omitting it starts an ad-hoc session. performed_on defaults to today server-side.
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

// SessionMovementPatch checks off / records actuals / swaps one entry. movement_id
// re-points the entry (the mid-session swap), carrying the actuals to the substitute.
export interface SessionMovementPatch {
  done?: boolean
  actual_sets?: number | null
  actual_reps?: string | null
  actual_load?: string | null
  notes?: string
  movement_id?: number
}

// SessionMovementInput appends one movement to a session already underway — the
// ad-hoc logging path, where the session starts empty and grows as things get done.
export interface SessionMovementInput {
  movement_id: number
  done?: boolean
  actual_sets?: number | null
  actual_reps?: string | null
  actual_load?: string | null
  notes?: string
}

// SessionPromote turns what was performed ad-hoc into a reusable workout: the logged
// actuals become the prescription, so only the labelling is supplied here.
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
}

function queryString(filter: SessionFilter): string {
  const params = new URLSearchParams()
  if (filter.workout_id !== undefined) params.set('workout_id', String(filter.workout_id))
  if (filter.from) params.set('from', filter.from)
  if (filter.to) params.set('to', filter.to)
  const q = params.toString()
  return q ? `?${q}` : ''
}

export const sessionsApi = {
  list: (filter: SessionFilter = {}) => http.get<Session[]>(`/sessions${queryString(filter)}`),
  get: (id: string) => http.get<Session>(`/sessions/${encodeURIComponent(id)}`),
  create: (body: SessionCreate) => http.post<Session>('/sessions', body),
  update: (id: string, body: SessionUpdate) => http.put<Session>(`/sessions/${encodeURIComponent(id)}`, body),
  remove: (id: string) => http.del(`/sessions/${encodeURIComponent(id)}`),
  // Per-movement logging — each write returns the refreshed session, so the logging
  // screen re-renders (including the new entry's previous actuals) from one response.
  addMovement: (id: string, body: SessionMovementInput) =>
    http.post<Session>(`/sessions/${encodeURIComponent(id)}/movements`, body),
  updateMovement: (id: string, entryId: number, body: SessionMovementPatch) =>
    http.patch<Session>(`/sessions/${encodeURIComponent(id)}/movements/${entryId}`, body),
  removeMovement: (id: string, entryId: number) =>
    http.delData<Session>(`/sessions/${encodeURIComponent(id)}/movements/${entryId}`),
  // Promotion returns the created workout, not the session.
  promote: (id: string, body: SessionPromote) => http.post<Workout>(`/sessions/${encodeURIComponent(id)}/workout`, body),
}

// doneCount reports how many of a session's movements are checked off.
export function doneCount(session: Session): number {
  return session.movements.filter((m) => m.done).length
}

// actualsSummary renders an entry's logged sets/reps/load compactly (e.g. "5 × 5 · 100lb").
export function actualsSummary(m: SessionMovement): string {
  const parts: string[] = []
  if (m.actual_sets != null || m.actual_reps) parts.push(`${m.actual_sets ?? '?'} × ${m.actual_reps ?? '?'}`)
  if (m.actual_load) parts.push(m.actual_load)
  return parts.join(' · ')
}

// previousSummary renders the last performed result as one line for the logging screen
// (e.g. "5 × 5 · 185lb · 2026-07-08"). Empty when the movement has never been performed,
// so the caller can skip the row entirely rather than render a placeholder.
export function previousSummary(m: SessionMovement): string {
  const p = m.previous
  if (!p) return ''
  const parts: string[] = []
  if (p.actual_sets != null || p.actual_reps) parts.push(`${p.actual_sets ?? '?'} × ${p.actual_reps ?? '?'}`)
  if (p.actual_load) parts.push(p.actual_load)
  parts.push(p.performed_on)
  return parts.join(' · ')
}
