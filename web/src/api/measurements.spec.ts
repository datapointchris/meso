import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  metricsApi,
  measurementsApi,
  statsApi,
  changeVerdict,
  formatValue,
  slugifyMetricName,
  type MetricTrend,
} from './measurements'
import { ApiError } from './client'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
  } as Response
}

function trend(partial: Partial<MetricTrend>): MetricTrend {
  return {
    metric: 'x',
    label: 'X',
    unit: 'lb',
    direction: 'higher_better',
    category: 'strength',
    points: [],
    first: null,
    latest: null,
    change: null,
    count: 0,
    ...partial,
  }
}

describe('measurements api', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it('builds a filtered measurement list query', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await measurementsApi.list({ metric: '5k-time', from: '2026-07-01' })
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/measurements?')
    expect(url).toContain('metric=5k-time')
    expect(url).toContain('from=2026-07-01')
  })

  it('omits the query string when no filter is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse([]))
    await measurementsApi.list()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/measurements')
  })

  it('records a measurement as a JSON POST', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 1, metric: '5k-time', value: 1650 }))
    await measurementsApi.record({ metric: '5k-time', value: 1650, measured_on: '2026-07-27' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/measurements')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({ metric: '5k-time', value: 1650, measured_on: '2026-07-27' })
  })

  it('builds a trend path with an encoded metric name and window', async () => {
    fetchMock.mockResolvedValue(jsonResponse(trend({ metric: '5k-time', count: 0 })))
    await metricsApi.trend('5k-time', { from: '2026-07-01' })
    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/metrics/5k-time/trend?')
    expect(url).toContain('from=2026-07-01')
  })

  it('defines a metric via POST /metrics', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ name: 'toe-reach' }))
    await metricsApi.define({ name: 'toe-reach', unit: 'cm', direction: 'higher_better', category: 'mobility' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/metrics')
    expect(init.method).toBe('POST')
  })

  it('updates a metric via PUT with an encoded name', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ name: 'row-machine-500m' }))
    await metricsApi.update('row-machine-500m', { label: 'Row 500m' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/metrics/row-machine-500m')
    expect(init.method).toBe('PUT')
    expect(init.body).toBe(JSON.stringify({ label: 'Row 500m' }))
  })

  it('deletes a metric via DELETE', async () => {
    fetchMock.mockResolvedValue(jsonResponse({}, 204))
    await metricsApi.remove('5k-time')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/metrics/5k-time')
    expect(init.method).toBe('DELETE')
  })

  it('fetches the stats summary', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ metrics: [], library: {}, sessions: {} }))
    await statsApi.get()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/stats')
  })

  it('throws a typed ApiError carrying the server message on non-2xx', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Conflict', message: 'unknown metric' }, 409))
    await expect(measurementsApi.record({ metric: 'ghost', value: 1 })).rejects.toMatchObject({
      status: 409,
      message: 'unknown metric',
    })
    fetchMock.mockResolvedValue(jsonResponse({ error: 'Conflict', message: 'unknown metric' }, 409))
    await expect(measurementsApi.record({ metric: 'ghost', value: 1 })).rejects.toBeInstanceOf(ApiError)
  })
})

describe('changeVerdict', () => {
  it('reads a rising higher_better metric as improved', () => {
    expect(changeVerdict(trend({ direction: 'higher_better', change: 40 }))).toBe('improved')
  })
  it('reads a falling lower_better metric (faster 5k) as improved', () => {
    expect(changeVerdict(trend({ direction: 'lower_better', change: -150 }))).toBe('improved')
  })
  it('reads a falling higher_better metric as worse', () => {
    expect(changeVerdict(trend({ direction: 'higher_better', change: -10 }))).toBe('worse')
  })
  it('treats a body metric as neutral regardless of sign', () => {
    expect(changeVerdict(trend({ category: 'body', direction: 'higher_better', change: 3 }))).toBe('flat')
  })
  it('is flat for zero change and null for an unmeasured metric', () => {
    expect(changeVerdict(trend({ change: 0 }))).toBe('flat')
    expect(changeVerdict(trend({ change: null }))).toBe(null)
  })
})

describe('slugifyMetricName', () => {
  it('derives the API key from a typed display label', () => {
    expect(slugifyMetricName('Row Machine 500m')).toBe('row-machine-500m')
  })
  it('collapses punctuation and trims stray separators', () => {
    expect(slugifyMetricName('  Back Squat — working weight!  ')).toBe('back-squat-working-weight')
  })
})

describe('formatValue', () => {
  it('drops a trailing .0 but keeps real decimals', () => {
    expect(formatValue(185)).toBe('185')
    expect(formatValue(185.0)).toBe('185')
    expect(formatValue(202.5)).toBe('202.5')
  })
})
