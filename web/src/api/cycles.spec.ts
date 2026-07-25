import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  cyclesApi,
  reviewApi,
  cycleTargetSummary,
  cyclePrescriptionSummary,
  type Cycle,
  type CycleWorkout,
} from './cycles'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

describe('cyclesApi', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds a filtered list query and hits /api/v1/cycles', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await cyclesApi.list({ status: 'active', search: '5k' })

    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/cycles?')
    expect(url).toContain('status=active')
    expect(url).toContain('search=5k')
  })

  it('omits the query string entirely when no filter is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await cyclesApi.list()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/cycles')
  })

  it('PATCHes a workout swap (only workout_id changes)', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, workouts: [] }))
    await cyclesApi.updateWorkout(1, 4, { workout_id: 9 })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/cycles/1/workouts/4')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body as string)).toEqual({ workout_id: 9 })
  })

  it('PATCHes the collection to reorder', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, workouts: [] }))
    await cyclesApi.reorderWorkouts(1, [4, 2, 3])
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/cycles/1/workouts')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body as string)).toEqual({ entry_ids: [4, 2, 3] })
  })

  it('DELETEs an entry via delData and decodes the refreshed cycle', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, name: 'Block 1', workouts: [] }))
    const c = await cyclesApi.removeWorkout(1, 4)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/cycles/1/workouts/4')
    expect(init.method).toBe('DELETE')
    expect(c.name).toBe('Block 1')
  })

  it('throws a typed ApiError carrying the server message on non-2xx', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Conflict', message: 'already exists' }, 409))
    await expect(cyclesApi.create({ name: 'Dup' })).rejects.toMatchObject({ status: 409, message: 'already exists' })
    await expect(cyclesApi.create({ name: 'Dup' })).rejects.toBeInstanceOf(ApiError)
  })
})

describe('reviewApi', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('passes the since window and hits /api/v1/review', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ since: '2026-05-01', active_cycles: [], sessions: [], measurements: [], log_entries: [] }),
    )
    await reviewApi.get('12w')
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/review?')
    expect(url).toContain('since=12w')
  })

  it('omits the query string when no window is given', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ since: '2026-06-24', active_cycles: [], sessions: [], measurements: [], log_entries: [] }),
    )
    await reviewApi.get()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/review')
  })
})

describe('cycleTargetSummary', () => {
  const base: Cycle = {
    id: 1,
    name: 'Block',
    goal_summary: '',
    target_metric: null,
    target_value: null,
    target_date: null,
    start_date: null,
    status: 'planned',
    notes: '',
    workouts: [],
    created_at: '',
    updated_at: '',
  }

  it('renders metric → value when both are set', () => {
    expect(cycleTargetSummary({ ...base, target_metric: 'deadlift', target_value: 315 })).toBe('deadlift → 315')
  })

  it('renders just the metric when there is no value', () => {
    expect(cycleTargetSummary({ ...base, target_metric: 'deadlift' })).toBe('deadlift')
  })

  it('is empty when there is no target', () => {
    expect(cycleTargetSummary(base)).toBe('')
  })
})

describe('cyclePrescriptionSummary', () => {
  const base: CycleWorkout = {
    id: 1,
    workout_id: 7,
    workout_name: 'Base Week',
    workout_theme: null,
    position: 1,
    week: null,
    phase: null,
    frequency: null,
    intensity: null,
    conditions: null,
  }

  it('joins week · phase · frequency · intensity', () => {
    expect(
      cyclePrescriptionSummary({ ...base, week: 1, phase: 'base', frequency: '3×/week', intensity: 'easy / Zone 2' }),
    ).toBe('wk 1 · base · 3×/week · easy / Zone 2')
  })

  it('is empty when nothing is prescribed', () => {
    expect(cyclePrescriptionSummary(base)).toBe('')
  })
})
