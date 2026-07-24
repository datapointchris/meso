// The SPA's HTTP layer. All requests go to same-origin `/api/v1/*`: in dev the
// Vite proxy forwards to the Go API, in production Caddy owns the split. Auth is
// Authelia cookie SSO at the edge, so requests carry credentials and never an
// app-issued token — there is no login screen in meso itself.

const BASE = '/api/v1'

// ApiError carries the HTTP status and the API's {error,message} envelope so
// callers can branch (401 → the edge bounced us, 404 → missing) and show the
// server's message.
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
    // Send the Authelia session cookie with every request.
    credentials: 'include',
    headers: { Accept: 'application/json' },
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
  del: (path: string) => request<void>('DELETE', path),
}
