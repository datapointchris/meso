// Typed access to the sessions endpoints and their per-movement logging sub-resource.
// Types mirror the Go API's JSON wire contract; the DB owns id, timestamps, and the
// UUID7 session id (treated here as an opaque string).
import { http } from './client'
import type { MovementKind } from './movements'

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
  // Per-movement logging — each PATCH returns the refreshed session.
  updateMovement: (id: string, entryId: number, body: SessionMovementPatch) =>
    http.patch<Session>(`/sessions/${encodeURIComponent(id)}/movements/${entryId}`, body),
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
