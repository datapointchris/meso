// Typed access to the movements + muscles endpoints. Types mirror the Go API's
// JSON wire contract; the DB owns id and timestamps.
import { http } from './client'

export type MovementKind = 'exercise' | 'stretch' | 'yoga_pose'
// LoadMode says how a movement is loaded, which decides what the logging screen asks
// for. movement_kind cannot stand in — a back squat and a nordic curl are both
// 'exercise' — and neither can equipment, which is free-form and empty by default.
export type LoadMode = 'weighted' | 'bodyweight' | 'timed' | 'assisted'
export type MuscleRole = 'primary' | 'secondary'
export type RelationshipKind = 'alternate' | 'antagonist' | 'progression' | 'regression' | 'see_also'

export interface MovementMuscle {
  muscle: string
  region: string
  role: MuscleRole
}

// RelatedMovement is a movement reached through a relationship, embedded on detail.
export interface RelatedMovement {
  id: number
  name: string
  movement_kind: MovementKind
  relationship_kind: RelationshipKind
  favorite: boolean
}

export interface Movement {
  id: number
  name: string
  movement_kind: MovementKind
  load_mode: LoadMode
  favorite: boolean
  rating: number | null
  tags: string[]
  equipment: string[]
  how_to: string
  form_cues: string
  common_faults: string
  default_sets: number | null
  default_reps: string | null
  default_hold_seconds: number | null
  sanskrit_name: string | null
  measurable_rom: boolean
  source_url: string | null
  source_name: string | null
  muscles: MovementMuscle[]
  related: RelatedMovement[]
  created_at: string
  updated_at: string
}

export interface Muscle {
  name: string
  region: string
}

export interface MuscleInput {
  muscle: string
  role: MuscleRole
}

// MovementWrite is the create/update payload. Fields omitted from an update are
// left unchanged server-side, so update senders should pass only what changed.
export interface MovementWrite {
  name?: string
  movement_kind?: MovementKind
  load_mode?: LoadMode
  favorite?: boolean
  rating?: number | null
  tags?: string[]
  equipment?: string[]
  how_to?: string
  form_cues?: string
  common_faults?: string
  default_sets?: number | null
  default_reps?: string | null
  default_hold_seconds?: number | null
  sanskrit_name?: string | null
  measurable_rom?: boolean
  source_url?: string | null
  source_name?: string | null
  muscles?: MuscleInput[]
}

export interface MovementFilter {
  kind?: MovementKind | ''
  load_mode?: LoadMode | ''
  favorite?: boolean
  tag?: string
  equipment?: string
  muscle?: string
  region?: string
  search?: string
}

function queryString(filter: MovementFilter): string {
  const params = new URLSearchParams()
  if (filter.kind) params.set('kind', filter.kind)
  if (filter.load_mode) params.set('load_mode', filter.load_mode)
  if (filter.favorite !== undefined) params.set('favorite', String(filter.favorite))
  if (filter.tag) params.set('tag', filter.tag)
  if (filter.equipment) params.set('equipment', filter.equipment)
  if (filter.muscle) params.set('muscle', filter.muscle)
  if (filter.region) params.set('region', filter.region)
  if (filter.search) params.set('search', filter.search)
  const q = params.toString()
  return q ? `?${q}` : ''
}

export interface RelationshipInput {
  related_movement_id: number
  relationship_kind: RelationshipKind
}

export const movementsApi = {
  list: (filter: MovementFilter = {}) => http.get<Movement[]>(`/movements${queryString(filter)}`),
  get: (id: number) => http.get<Movement>(`/movements/${id}`),
  create: (body: MovementWrite) => http.post<Movement>('/movements', body),
  update: (id: number, body: MovementWrite) => http.put<Movement>(`/movements/${id}`, body),
  remove: (id: number) => http.del(`/movements/${id}`),
  muscles: () => http.get<Muscle[]>('/muscles'),
  // Relationships return the refreshed movement, so the detail view re-renders its
  // related list without a follow-up fetch.
  addRelated: (id: number, body: RelationshipInput) => http.post<Movement>(`/movements/${id}/related`, body),
  removeRelated: (id: number, relatedId: number, kind?: RelationshipKind) =>
    http.del(`/movements/${id}/related/${relatedId}${kind ? `?kind=${kind}` : ''}`),
}

// RELATIONSHIP_KIND_LABELS gives each relationship kind a human label.
export const RELATIONSHIP_KIND_LABELS: Record<RelationshipKind, string> = {
  alternate: 'Alternate',
  antagonist: 'Antagonist',
  progression: 'Progression',
  regression: 'Regression',
  see_also: 'See also',
}

// primaryMuscles returns the names of a movement's primary muscles.
export function primaryMuscles(m: Movement): string[] {
  return m.muscles.filter((mm) => mm.role === 'primary').map((mm) => mm.muscle)
}

// KIND_LABELS gives each kind a human label for chips and headings.
export const KIND_LABELS: Record<MovementKind, string> = {
  exercise: 'Exercise',
  stretch: 'Stretch',
  yoga_pose: 'Yoga Pose',
}
