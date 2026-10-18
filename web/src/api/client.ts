// The SPA's HTTP layer. All requests go to same-origin `/api/v1/*`: in dev the
// Vite proxy forwards to the Go API, in production Caddy owns the split. The API
// logs the browser in itself and keeps the session, so the browser holds one
// HttpOnly cookie and this code never sees a token. A read that gets a 401
// sends the browser to the API's login route, which brings it back to the same
// page.

const BASE = '/api/v1'
const LOGIN = '/api/auth/login'

// A login that completes while the API still answers 401, as with cookies
// blocked, would send the browser round the provider forever. A person
// starting again from a refused login is a second page sending the browser to
// log in inside the window, so a third is the loop, and its reads fail instead.
const LOGIN_WINDOW_MS = 60_000
const LOGIN_STARTS_ALLOWED = 2
const LOGIN_STARTS_KEY = 'loginStarts'

const SESSION_ENDED = 'Your login has ended, so this change was not saved. Reload the page to log in again.'

// One start per page: the reads a page fires together all get the 401.
let loggingIn = false

function recentLoginStarts(now: number): number[] {
  try {
    const starts = JSON.parse(sessionStorage.getItem(LOGIN_STARTS_KEY) ?? '[]') as number[]
    return starts.filter((at) => now - at < LOGIN_WINDOW_MS)
  } catch {
    return []
  }
}

function sendToLogin(): boolean {
  if (loggingIn) return true
  const now = Date.now()
  const recent = recentLoginStarts(now)
  if (recent.length >= LOGIN_STARTS_ALLOWED) return false
  sessionStorage.setItem(LOGIN_STARTS_KEY, JSON.stringify([...recent, now]))
  loggingIn = true
  const here = location.pathname + location.search + location.hash
  location.assign(`${LOGIN}?return_to=${encodeURIComponent(here)}`)
  return true
}

// ApiError carries the HTTP status and the API's {error,message} envelope so
// callers can branch (401 → the login did not take, 404 → missing) and show
// the server's message.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    // Send the session cookie the API set at login with every request.
    credentials: 'include',
    // The API refuses its session cookie on a request without X-CSRF, which
    // another origin's page cannot add without a preflight it never gets.
    headers: { Accept: 'application/json', 'X-CSRF': '1' },
  }
  if (body !== undefined) {
    init.headers = { ...init.headers, 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }

  let resp: Response
  try {
    resp = await fetch(BASE + path, init)
  } catch (cause) {
    throw new ApiError(0, `could not reach the meso API: ${(cause as Error).message}`)
  }

  if (resp.status === 401) {
    // A write fails in place so its caller rolls back; navigating would drop the change.
    if (method !== 'GET') throw new ApiError(401, SESSION_ENDED)
    // The page is navigating away, so the caller never settles and shows no error.
    if (sendToLogin()) return new Promise<T>(() => {})
  }
  if (!resp.ok) {
    throw new ApiError(resp.status, await errorMessage(resp))
  }
  if (resp.status === 204) {
    return undefined as T
  }
  return (await resp.json()) as T
}

// errorMessage best-effort extracts the API's {error,message} body, falling back
// to the status text for a non-JSON body (a proxy page, an Authelia redirect).
async function errorMessage(resp: Response): Promise<string> {
  try {
    const data = (await resp.json()) as { message?: string; error?: string }
    return data.message || data.error || resp.statusText
  } catch {
    return resp.statusText
  }
}

export const http = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
  patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
  del: (path: string) => request<void>('DELETE', path),
  // delData is a DELETE whose 200 response carries a body (e.g. removing a workout
  // entry returns the refreshed workout), distinct from del which expects 204.
  delData: <T>(path: string) => request<T>('DELETE', path),
}
