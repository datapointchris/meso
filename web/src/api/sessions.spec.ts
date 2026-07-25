import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  sessionsApi,
  doneCount,
  actualsSummary,
  previousSummary,
  type Session,
  type SessionMovement,
} from './sessions'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

describe('sessionsApi', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds a date-filtered list query and hits /api/v1/sessions', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await sessionsApi.list({ from: '2026-07-01', to: '2026-07-31', workout_id: 3 })

    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/sessions?')
    expect(url).toContain('from=2026-07-01')
    expect(url).toContain('to=2026-07-31')
    expect(url).toContain('workout_id=3')
  })

  it('omits the query string entirely when no filter is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await sessionsApi.list()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/sessions')
  })

  it('POSTs a start-from-workout create', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', workout_id: 1, movements: [] }, 201))
    await sessionsApi.create({ workout_id: 1 })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/sessions')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({ workout_id: 1 })
  })

  it('PATCHes a logged entry (done + actuals) at the sub-resource', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', movements: [] }))
    await sessionsApi.updateMovement('018f-aaa', 42, { done: true, actual_load: '100lb' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/sessions/018f-aaa/movements/42')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body as string)).toEqual({ done: true, actual_load: '100lb' })
  })

  it('encodes the session id in the path', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 'a b', movements: [] }))
    await sessionsApi.get('a b')
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/sessions/a%20b')
  })

  it('throws a typed ApiError carrying the server message on non-2xx', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Bad Request', message: 'invalid date' }, 400))
    await expect(sessionsApi.create({ performed_on: 'nope' })).rejects.toMatchObject({ status: 400, message: 'invalid date' })
    await expect(sessionsApi.create({ performed_on: 'nope' })).rejects.toBeInstanceOf(ApiError)
  })
})

describe('session helpers', () => {
  const entry: SessionMovement = {
    id: 1,
    movement_id: 7,
    movement_name: 'Bench Press',
    movement_kind: 'exercise',
    position: 1,
    done: false,
    actual_sets: null,
    actual_reps: null,
    actual_load: null,
    previous: null,
    notes: '',
  }

  it('doneCount counts checked-off movements', () => {
    const session = {
      movements: [
        { ...entry, id: 1, done: true },
        { ...entry, id: 2, done: false },
        { ...entry, id: 3, done: true },
      ],
    } as Session
    expect(doneCount(session)).toBe(2)
  })

  it('actualsSummary renders sets × reps · load', () => {
    expect(actualsSummary({ ...entry, actual_sets: 5, actual_reps: '5', actual_load: '100lb' })).toBe('5 × 5 · 100lb')
  })

  it('actualsSummary is empty when nothing is logged', () => {
    expect(actualsSummary(entry)).toBe('')
  })

  it('previousSummary renders the last result with its date', () => {
    const withPrevious: SessionMovement = {
      ...entry,
      previous: { performed_on: '2026-07-08', actual_sets: 5, actual_reps: '5', actual_load: '185lb' },
    }
    expect(previousSummary(withPrevious)).toBe('5 × 5 · 185lb · 2026-07-08')
  })

  it('previousSummary is empty when the movement has never been performed', () => {
    expect(previousSummary(entry)).toBe('')
  })

  it('previousSummary still dates a result logged without numbers', () => {
    const noNumbers: SessionMovement = {
      ...entry,
      previous: { performed_on: '2026-07-08', actual_sets: null, actual_reps: null, actual_load: null },
    }
    expect(previousSummary(noNumbers)).toBe('2026-07-08')
  })
})
