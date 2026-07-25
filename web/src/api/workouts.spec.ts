import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { workoutsApi, prescriptionSummary, type WorkoutMovement } from './workouts'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

describe('workoutsApi', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds a filtered list query and hits /api/v1/workouts', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await workoutsApi.list({ theme: 'push', favorite: true })

    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/workouts?')
    expect(url).toContain('theme=push')
    expect(url).toContain('favorite=true')
  })

  it('omits the query string entirely when no filter is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await workoutsApi.list()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/workouts')
  })

  it('PATCHes a movement swap (only movement_id changes)', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, movements: [] }))
    await workoutsApi.updateMovement(1, 4, { movement_id: 9 })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/workouts/1/movements/4')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body as string)).toEqual({ movement_id: 9 })
  })

  it('PATCHes the collection to reorder', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, movements: [] }))
    await workoutsApi.reorderMovements(1, [4, 2, 3])
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/workouts/1/movements')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body as string)).toEqual({ entry_ids: [4, 2, 3] })
  })

  it('DELETEs an entry via delData and decodes the refreshed workout', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, name: 'Push', movements: [] }))
    const w = await workoutsApi.removeMovement(1, 4)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/workouts/1/movements/4')
    expect(init.method).toBe('DELETE')
    expect(w.name).toBe('Push')
  })

  it('throws a typed ApiError carrying the server message on non-2xx', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Conflict', message: 'already exists' }, 409))
    await expect(workoutsApi.create({ name: 'Dup' })).rejects.toMatchObject({ status: 409, message: 'already exists' })
    await expect(workoutsApi.create({ name: 'Dup' })).rejects.toBeInstanceOf(ApiError)
  })
})

describe('prescriptionSummary', () => {
  const base: WorkoutMovement = {
    id: 1,
    movement_id: 7,
    movement_name: 'Bench Press',
    movement_kind: 'exercise',
    position: 1,
    sets: null,
    reps: null,
    load: null,
    rest_seconds: null,
    superset_group: null,
    notes: '',
  }

  it('renders sets × reps · load · rest', () => {
    expect(prescriptionSummary({ ...base, sets: 5, reps: '5', load: '80% 1RM', rest_seconds: 120 })).toBe(
      '5 × 5 · 80% 1RM · rest 120s',
    )
  })

  it('is empty when no prescription is set', () => {
    expect(prescriptionSummary(base)).toBe('')
  })
})
