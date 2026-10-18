import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, http } from './client'

function stubFetch(status = 200): ReturnType<typeof vi.fn> {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: status < 400,
    status,
    statusText: '',
    json: async () => ({}),
  } as Response)
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => {
  vi.unstubAllGlobals()
  sessionStorage.clear()
})

describe('request', () => {
  it('sends the header the API requires beside its session cookie', async () => {
    const fetchMock = stubFetch()
    await http.get('/cycles')
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect((init.headers as Record<string, string>)['X-CSRF']).toBe('1')
  })

  // A person starting again from a refused login is the second start. A third
  // inside the minute is a login that completes without taking, which would
  // otherwise loop through the provider forever.
  it('sends the browser to log in on a 401, twice a minute at most', async () => {
    stubFetch(401)
    const assign = vi.fn()
    vi.stubGlobal('location', { pathname: '/sessions/3', search: '', hash: '', assign })

    void http.get('/cycles')
    void http.get('/cycles')
    await vi.waitFor(() => expect(assign).toHaveBeenCalledTimes(2))
    expect(assign).toHaveBeenCalledWith('/api/auth/login?return_to=%2Fsessions%2F3')
    await expect(http.get('/cycles')).rejects.toThrow(ApiError)
    expect(assign).toHaveBeenCalledTimes(2)
  })
})
