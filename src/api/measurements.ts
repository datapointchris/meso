// Typed access to the metrics, measurements, and stats endpoints. Types mirror the
// Go API's JSON wire contract; the DB owns id, timestamps, and source defaults.
import { http } from './client'

export type MetricDirection = 'higher_better' | 'lower_better'
export type MetricCategory = 'strength' | 'cardio' | 'mobility' | 'body'

// MetricDefinition names a tracked metric and carries the metadata a chart needs:
// its unit, which direction is improvement, and a category for grouping.
export interface MetricDefinition {
  name: string
  unit: string
  direction: MetricDirection
  category: MetricCategory
  created_at: string
}

export interface MetricDefinitionCreate {
  name: string
  unit: string
  direction: MetricDirection
  category: MetricCategory
}

// Measurement is one dated reading of a metric — a point in the time series.
export interface Measurement {
  id: number
  metric: string
  value: number
  measured_on: string
  source: string
  notes: string
  created_at: string
}

// MeasurementCreate records a reading. measured_on defaults to today server-side;
// source defaults to "manual".
export interface MeasurementCreate {
  metric: string
  value: number
  measured_on?: string
  notes?: string
}

export interface MeasurementUpdate {
  value?: number
  measured_on?: string
  notes?: string
}

export interface TrendPoint {
  measured_on: string
  value: number
}

// MetricTrend is a metric's full series (oldest-first) plus the summary numbers a
// card shows. first/latest/change are null for a defined-but-unmeasured metric.
export interface MetricTrend {
  metric: string
  unit: string
  direction: MetricDirection
  category: MetricCategory
  points: TrendPoint[]
  first: number | null
  latest: number | null
  change: number | null
  count: number
}

export interface KindCount {
  kind: string
  count: number
}

export interface WeekCount {
  week_start: string
  count: number
}

export interface LibraryStats {
  by_kind: KindCount[]
  total_movements: number
  favorites: number
}

export interface SessionStats {
  by_week: WeekCount[]
  total: number
  last_30_days: number
}

// Stats is the whole stats-page payload — one GET renders the page.
export interface Stats {
  metrics: MetricTrend[]
  library: LibraryStats
  sessions: SessionStats
}

export interface MeasurementFilter {
  metric?: string
  from?: string
  to?: string
}

function queryString(filter: MeasurementFilter): string {
  const params = new URLSearchParams()
  if (filter.metric) params.set('metric', filter.metric)
  if (filter.from) params.set('from', filter.from)
  if (filter.to) params.set('to', filter.to)
  const q = params.toString()
  return q ? `?${q}` : ''
}

export const metricsApi = {
  list: () => http.get<MetricDefinition[]>('/metrics'),
  define: (body: MetricDefinitionCreate) => http.post<MetricDefinition>('/metrics', body),
  trend: (name: string, window: { from?: string; to?: string } = {}) => {
    const params = new URLSearchParams()
    if (window.from) params.set('from', window.from)
    if (window.to) params.set('to', window.to)
    const q = params.toString()
    return http.get<MetricTrend>(`/metrics/${encodeURIComponent(name)}/trend${q ? `?${q}` : ''}`)
  },
}

export const measurementsApi = {
  list: (filter: MeasurementFilter = {}) => http.get<Measurement[]>(`/measurements${queryString(filter)}`),
  get: (id: number) => http.get<Measurement>(`/measurements/${id}`),
  record: (body: MeasurementCreate) => http.post<Measurement>('/measurements', body),
  update: (id: number, body: MeasurementUpdate) => http.put<Measurement>(`/measurements/${id}`, body),
  remove: (id: number) => http.del(`/measurements/${id}`),
}

export const statsApi = {
  get: () => http.get<Stats>('/stats'),
}

export const CATEGORY_LABELS: Record<MetricCategory, string> = {
  strength: 'Strength',
  cardio: 'Cardio',
  mobility: 'Mobility',
  body: 'Body',
}

export type ChangeVerdict = 'improved' | 'worse' | 'flat'

// changeVerdict maps a trend's net change to improvement, worse, or flat — using
// direction so a smaller 5k time reads as an improvement. A "body" metric has no
// inherent good/bad direction, so its change is always neutral ("flat"). Null for an
// unmeasured metric.
export function changeVerdict(trend: MetricTrend): ChangeVerdict | null {
  if (trend.change === null) return null
  if (trend.change === 0 || trend.category === 'body') return 'flat'
  const wentUp = trend.change > 0
  const isGood = wentUp === (trend.direction === 'higher_better')
  return isGood ? 'improved' : 'worse'
}

// formatValue renders a measurement number for display. JS already drops a trailing
// ".0" (185.0 === 185) and keeps real decimals (202.5), so String() is the format —
// the named helper keeps the intent legible at call sites.
export function formatValue(value: number): string {
  return String(value)
}
