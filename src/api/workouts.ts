// Typed access to the workouts endpoints and their composition sub-resource. Types
// mirror the Go API's JSON wire contract; the DB owns id and timestamps.
import { http } from './client'
import type { MovementKind } from './movements'

// WorkoutMovement is one prescribed entry, with the movement's name/kind embedded
// for render. Prescription fields are nullable (a bodyweight circuit may set none).
export interface WorkoutMovement {
  id: number
  movement_id: number
  movement_name: string
  movement_kind: MovementKind
  position: number
  sets: number | null
  reps: string | null
  load: string | null
  rest_seconds: number | null
  superset_group: string | null
  notes: string
}

export interface Workout {
  id: number
  name: string
  theme: string | null
  tags: string[]
  notes: string
  favorite: boolean
  estimated_minutes: number | null
  movements: WorkoutMovement[]
  created_at: string
  updated_at: string
}

// WorkoutWrite is the create/update payload for workout-level fields. Fields omitted
// from an update are left unchanged server-side.
export interface WorkoutWrite {
  name?: string
  theme?: string | null
  tags?: string[]
  notes?: string
  favorite?: boolean
  estimated_minutes?: number | null
}

// WorkoutMovementInput is the write shape for adding an entry.
export interface WorkoutMovementInput {
  movement_id: number
  sets?: number | null
  reps?: string | null
  load?: string | null
  rest_seconds?: number | null
  superset_group?: string | null
  notes?: string
}

// WorkoutMovementPatch edits or swaps one entry. movement_id re-points the entry
// (the swap), carrying the untouched prescription to the substitute.
export interface WorkoutMovementPatch {
  movement_id?: number
  sets?: number | null
  reps?: string | null
  load?: string | null
  rest_seconds?: number | null
  superset_group?: string | null
  notes?: string
}

export interface WorkoutFilter {
  theme?: string
  tag?: string
  search?: string
  favorite?: boolean
}

function queryString(filter: WorkoutFilter): string {
  const params = new URLSearchParams()
  if (filter.theme) params.set('theme', filter.theme)
  if (filter.tag) params.set('tag', filter.tag)
  if (filter.search) params.set('search', filter.search)
  if (filter.favorite !== undefined) params.set('favorite', String(filter.favorite))
  const q = params.toString()
  return q ? `?${q}` : ''
}

export const workoutsApi = {
  list: (filter: WorkoutFilter = {}) => http.get<Workout[]>(`/workouts${queryString(filter)}`),
  get: (id: number) => http.get<Workout>(`/workouts/${id}`),
  create: (body: WorkoutWrite & { movements?: WorkoutMovementInput[] }) => http.post<Workout>('/workouts', body),
  update: (id: number, body: WorkoutWrite) => http.put<Workout>(`/workouts/${id}`, body),
  remove: (id: number) => http.del(`/workouts/${id}`),
  // Composition sub-resource — each write returns the refreshed workout.
  addMovement: (id: number, body: WorkoutMovementInput) => http.post<Workout>(`/workouts/${id}/movements`, body),
  updateMovement: (id: number, entryId: number, body: WorkoutMovementPatch) =>
    http.patch<Workout>(`/workouts/${id}/movements/${entryId}`, body),
  reorderMovements: (id: number, entryIds: number[]) =>
    http.patch<Workout>(`/workouts/${id}/movements`, { entry_ids: entryIds }),
  removeMovement: (id: number, entryId: number) => http.delData<Workout>(`/workouts/${id}/movements/${entryId}`),
}

// prescriptionSummary renders an entry's sets/reps/load compactly (e.g. "5 × 5 · 80% 1RM").
export function prescriptionSummary(m: WorkoutMovement): string {
  const parts: string[] = []
  if (m.sets != null || m.reps) parts.push(`${m.sets ?? '?'} × ${m.reps ?? '?'}`)
  if (m.load) parts.push(m.load)
  if (m.rest_seconds != null) parts.push(`rest ${m.rest_seconds}s`)
  return parts.join(' · ')
}
