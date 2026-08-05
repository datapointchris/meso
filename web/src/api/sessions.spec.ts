import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  sessionsApi,
  setCount,
  isInProgress,
  targetSummary,
  previousSummary,
  type Session,
  type SessionMovement,
  type SessionSet,
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

  it('PATCHes a logged entry (done + target) at the sub-resource', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', movements: [] }))
    await sessionsApi.updateMovement('018f-aaa', 42, { done: true, target_load: '100lb' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/sessions/018f-aaa/movements/42')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body as string)).toEqual({ done: true, target_load: '100lb' })
  })

  // The one-tap path: an empty body is what tells the server to repeat the last set, so
  // the client must not fill it in with anything.
  it('POSTs an empty body to log a set by default', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', movements: [] }, 201))
    await sessionsApi.addSet('018f-aaa', 42)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/sessions/018f-aaa/movements/42/sets')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({})
  })

  it('addresses one set for update and removal', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', movements: [] }))
    await sessionsApi.updateSet('018f-aaa', 42, 7, { reps: 6 })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/sessions/018f-aaa/movements/42/sets/7')

    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', movements: [] }))
    await sessionsApi.removeSet('018f-aaa', 42, 7)
    expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/sessions/018f-aaa/movements/42/sets/7')
    expect((fetchMock.mock.calls[1][1] as RequestInit).method).toBe('DELETE')
  })

  it('POSTs to finish', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f-aaa', movements: [] }))
    await sessionsApi.finish('018f-aaa')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/sessions/018f-aaa/finish')
    expect(init.method).toBe('POST')
  })

  it('asks for only unfinished sessions when told to', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await sessionsApi.list({ unfinished: true })
    expect(fetchMock.mock.calls[0][0]).toContain('unfinished=true')
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
    load_mode: 'weighted',
    position: 1,
    done: false,
    target_sets: null,
    target_reps: null,
    target_load: null,
    sets: [],
    previous: null,
    notes: '',
  }

  function set(over: Partial<SessionSet> = {}): SessionSet {
    return {
      id: 1,
      position: 1,
      reps: null,
      load: null,
      hold_seconds: null,
      set_kind: 'working',
      notes: '',
      logged_at: '2026-07-24T10:00:00Z',
      ...over,
    }
  }

  it('setCount totals the sets across every movement', () => {
    const session = {
      movements: [
        { ...entry, id: 1, sets: [set({ id: 1 }), set({ id: 2, position: 2 })] },
        { ...entry, id: 2, sets: [] },
        { ...entry, id: 3, sets: [set({ id: 3 })] },
      ],
    } as Session
    expect(setCount(session)).toBe(3)
  })

  it('isInProgress is what separates training now from history', () => {
    expect(isInProgress({ finished_at: null } as Session)).toBe(true)
    expect(isInProgress({ finished_at: '2026-07-24T11:00:00Z' } as Session)).toBe(false)
  })

  it('targetSummary renders the plan the entry is measured against', () => {
    expect(targetSummary({ ...entry, target_sets: 5, target_reps: '5', target_load: '100lb' })).toBe('5 × 5 · 100lb')
  })

  it('targetSummary is empty for a free-form entry, which has no plan', () => {
    expect(targetSummary(entry)).toBe('')
  })

  it('previousSummary renders the last result with its date', () => {
    const withPrevious: SessionMovement = {
      ...entry,
      previous: { performed_on: '2026-07-08', sets: 5, reps: 5, load: '185lb' },
    }
    expect(previousSummary(withPrevious)).toBe('5 × 5 · 185lb · 2026-07-08')
  })

  it('previousSummary is empty when the movement has never been performed', () => {
    expect(previousSummary(entry)).toBe('')
  })

  it('previousSummary still dates a result logged without numbers', () => {
    const noNumbers: SessionMovement = {
      ...entry,
      previous: { performed_on: '2026-07-08', sets: 2, reps: null, load: null },
    }
    expect(previousSummary(noNumbers)).toBe('2 × ? · 2026-07-08')
  })
})
