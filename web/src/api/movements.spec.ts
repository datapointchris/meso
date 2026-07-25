import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { movementsApi, primaryMuscles, type Movement } from './movements'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

const sample: Partial<Movement> = {
  id: 1,
  name: 'Deadlift',
  muscles: [
    { muscle: 'hamstrings', region: 'posterior', role: 'primary' },
    { muscle: 'quads', region: 'anterior', role: 'secondary' },
  ],
}

describe('movementsApi', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds a filtered list query and hits /api/v1/movements', async () => {
    fetchMock.mockResolvedValue(jsonResponse([sample]))
    await movementsApi.list({ kind: 'exercise', favorite: true, region: 'posterior' })

    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/movements?')
    expect(url).toContain('kind=exercise')
    expect(url).toContain('favorite=true')
    expect(url).toContain('region=posterior')
  })

  it('omits the query string entirely when no filter is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await movementsApi.list()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/movements')
  })

  it('sends credentials on every request (Authelia cookie)', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await movementsApi.list()
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(init.credentials).toBe('include')
  })

  it('PUTs an update as JSON', async () => {
    fetchMock.mockResolvedValue(jsonResponse(sample))
    await movementsApi.update(1, { favorite: true })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/movements/1')
    expect(init.method).toBe('PUT')
    expect(JSON.parse(init.body as string)).toEqual({ favorite: true })
  })

  it('throws a typed ApiError carrying the server message on non-2xx', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Conflict', message: 'already exists' }, 409))
    await expect(movementsApi.create({ name: 'Deadlift' })).rejects.toMatchObject({
      status: 409,
      message: 'already exists',
    })
    await expect(movementsApi.create({ name: 'Deadlift' })).rejects.toBeInstanceOf(ApiError)
  })
})

describe('primaryMuscles', () => {
  it('returns only the primary muscle names', () => {
    expect(primaryMuscles(sample as Movement)).toEqual(['hamstrings'])
  })
})
