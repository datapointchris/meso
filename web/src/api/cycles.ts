// Typed access to the cycles endpoints and their sequence sub-resource, plus the
// review capstone read. Types mirror the Go API's JSON wire contract; the DB owns id
// and timestamps.
import { http } from './client'
import type { Measurement } from './measurements'
import type { LogEntry } from './log'
import type { Session } from './sessions'

export type CycleStatus = 'planned' | 'active' | 'paused' | 'completed'

export const CYCLE_STATUSES: CycleStatus[] = ['planned', 'active', 'paused', 'completed']

// CycleWorkout is one entry in a cycle's ordered sequence, with the workout's
// name/theme embedded for render. The periodization fields are nullable.
export interface CycleWorkout {
  id: number
  workout_id: number
  workout_name: string
  workout_theme: string | null
  position: number
  week: number | null
  phase: string | null
  frequency: string | null
  intensity: string | null
  conditions: string | null
}

export interface Cycle {
  id: number
  name: string
  goal_summary: string
  target_metric: string | null
  target_value: number | null
  target_date: string | null
  start_date: string | null
  status: CycleStatus
  notes: string
  workouts: CycleWorkout[]
  created_at: string
  updated_at: string
}

// CycleWrite is the create/update payload for cycle-level fields. Fields omitted from
// an update are left unchanged; an explicit empty date/metric clears it.
export interface CycleWrite {
  name?: string
  goal_summary?: string
  target_metric?: string | null
  target_value?: number | null
  target_date?: string | null
  start_date?: string | null
  status?: CycleStatus
  notes?: string
}

// CycleWorkoutInput is the write shape for adding an entry to the sequence.
export interface CycleWorkoutInput {
  workout_id: number
  week?: number | null
  phase?: string | null
  frequency?: string | null
  intensity?: string | null
  conditions?: string | null
}

// CycleWorkoutPatch edits or swaps one entry. workout_id re-points the entry (the
// swap), carrying the untouched prescription to the substitute.
export interface CycleWorkoutPatch {
  workout_id?: number
  week?: number | null
  phase?: string | null
  frequency?: string | null
  intensity?: string | null
  conditions?: string | null
}

export interface CycleFilter {
  status?: CycleStatus
  search?: string
}

function queryString(filter: CycleFilter): string {
  const params = new URLSearchParams()
  if (filter.status) params.set('status', filter.status)
  if (filter.search) params.set('search', filter.search)
  const q = params.toString()
  return q ? `?${q}` : ''
}

export const cyclesApi = {
  list: (filter: CycleFilter = {}) => http.get<Cycle[]>(`/cycles${queryString(filter)}`),
  get: (id: number) => http.get<Cycle>(`/cycles/${id}`),
  create: (body: CycleWrite & { workouts?: CycleWorkoutInput[] }) => http.post<Cycle>('/cycles', body),
  update: (id: number, body: CycleWrite) => http.put<Cycle>(`/cycles/${id}`, body),
  remove: (id: number) => http.del(`/cycles/${id}`),
  // Sequence sub-resource — each write returns the refreshed cycle.
  addWorkout: (id: number, body: CycleWorkoutInput) => http.post<Cycle>(`/cycles/${id}/workouts`, body),
  updateWorkout: (id: number, entryId: number, body: CycleWorkoutPatch) =>
    http.patch<Cycle>(`/cycles/${id}/workouts/${entryId}`, body),
  reorderWorkouts: (id: number, entryIds: number[]) =>
    http.patch<Cycle>(`/cycles/${id}/workouts`, { entry_ids: entryIds }),
  removeWorkout: (id: number, entryId: number) => http.delData<Cycle>(`/cycles/${id}/workouts/${entryId}`),
}

// Review is the capstone read: active cycles plus recent sessions, measurements, and
// log entries in one window, the substrate for drafting the next cycle with Claude.
export interface Review {
  since: string
  active_cycles: Cycle[]
  sessions: Session[]
  measurements: Measurement[]
  log_entries: LogEntry[]
}

export const reviewApi = {
  get: (since?: string) => {
    const q = since ? `?${new URLSearchParams({ since }).toString()}` : ''
    return http.get<Review>(`/review${q}`)
  },
}

// cycleTargetSummary renders a cycle's metric target compactly (e.g.
// "deadlift-working-weight → 315"), or an empty string when there is no numeric target.
export function cycleTargetSummary(c: Cycle): string {
  if (!c.target_metric) return ''
  if (c.target_value == null) return c.target_metric
  return `${c.target_metric} → ${c.target_value}`
}

// cyclePrescriptionSummary renders an entry's week/phase/frequency/intensity compactly.
export function cyclePrescriptionSummary(cw: CycleWorkout): string {
  const parts: string[] = []
  if (cw.week != null) parts.push(`wk ${cw.week}`)
  if (cw.phase) parts.push(cw.phase)
  if (cw.frequency) parts.push(cw.frequency)
  if (cw.intensity) parts.push(cw.intensity)
  return parts.join(' · ')
}
