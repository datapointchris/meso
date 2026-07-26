import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { feedbackApi } from './feedback'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

describe('feedback api', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
    fetchMock.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('captures feedback as a JSON POST carrying the route', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: '019f', status: 'open' }, 201))
    await feedbackApi.capture({ body: 'Timer resets on sleep', context_path: '/sessions/3' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/feedback')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ body: 'Timer resets on sleep', context_path: '/sessions/3' }))
  })

  it('surfaces the server message when a capture is rejected', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ message: 'feedback body is required' }, 400))
    await expect(feedbackApi.capture({ body: '' })).rejects.toThrow(ApiError)
  })
})
