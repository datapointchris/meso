import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { logApi } from './log'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

describe('log api', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds a filtered list query', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await logApi.list({ from: '2026-07-01', tag: 'strength' })
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/log?')
    expect(url).toContain('from=2026-07-01')
    expect(url).toContain('tag=strength')
  })

  it('omits the query string when no filter is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await logApi.list()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/log')
  })

  it('creates an entry as a JSON POST', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f', entry_date: '2026-07-24', body: 'note' }))
    await logApi.create({ body: 'note', tags: ['rest'], mood: 'tired' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/log')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({ body: 'note', tags: ['rest'], mood: 'tired' })
  })

  it('updates an entry with an encoded id via PUT', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '018f', body: 'revised' }))
    await logApi.update('018f', { body: 'revised' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/log/018f')
    expect(init.method).toBe('PUT')
  })

  it('deletes an entry via DELETE', async () => {
    fetchMock.mockResolvedValue(jsonResponse(undefined, 204))
    await logApi.remove('018f')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/log/018f')
    expect(init.method).toBe('DELETE')
  })

  it('throws a typed ApiError carrying the server message on non-2xx', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Bad Request', message: 'invalid date' }, 400))
    await expect(logApi.create({ body: 'x', entry_date: 'nope' })).rejects.toMatchObject({
      status: 400,
      message: 'invalid date',
    })
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Bad Request', message: 'invalid date' }, 400))
    await expect(logApi.create({ body: 'x', entry_date: 'nope' })).rejects.toBeInstanceOf(ApiError)
  })
})
