import { afterEach, describe, expect, it, vi } from 'vitest'
import { http } from './client'

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

  // A page's reads are one start however many get a 401. A person starting
  // again from a refused login is the second page. A third inside the minute is
  // a login that completes without taking, which would otherwise loop through
  // the provider forever.
  it('sends the browser to log in once a page, from two pages a minute at most', async () => {
    stubFetch(401)
    const assign = vi.fn()
    vi.stubGlobal('location', { pathname: '/sessions/3', search: '', hash: '', assign })
    const page = async () => {
      vi.resetModules()
      return (await import('./client')).http
    }

    const first = await page()
    void first.get('/cycles')
    void first.get('/sessions')
    void first.get('/movements')
    await new Promise((settled) => setTimeout(settled, 0))
    expect(assign).toHaveBeenCalledTimes(1)
    expect(assign).toHaveBeenCalledWith('/api/auth/login?return_to=%2Fsessions%2F3')

    const second = await page()
    void second.get('/cycles')
    await vi.waitFor(() => expect(assign).toHaveBeenCalledTimes(2))

    const third = await page()
    await expect(third.get('/cycles')).rejects.toMatchObject({ status: 401 })
    expect(assign).toHaveBeenCalledTimes(2)
  })

  it('fails a write on a 401 and stays on the page', async () => {
    stubFetch(401)
    const assign = vi.fn()
    vi.stubGlobal('location', { pathname: '/sessions/3', search: '', hash: '', assign })

    await expect(http.post('/sessions', {})).rejects.toThrow('this change was not saved')
    expect(assign).not.toHaveBeenCalled()
  })
})
