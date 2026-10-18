// A read's time bounds as RFC 3339 instants: since inclusive, until exclusive, and an
// absent side open. The API maps each instant to a day in the offset it carries, so
// an instant built here carries the browser's offset.
export interface TimeWindow {
  since?: string
  until?: string
}

export function setWindow(params: URLSearchParams, bounds: TimeWindow): void {
  if (bounds.since) params.set('since', bounds.since)
  if (bounds.until) params.set('until', bounds.until)
}

// dayWindow turns a picked first and last day, each "YYYY-MM-DD" or empty, into a
// window holding both days whole.
export function dayWindow(first: string, last: string): TimeWindow {
  const bounds: TimeWindow = {}
  if (first) bounds.since = dayStart(first, 0)
  if (last) bounds.until = dayStart(last, 1)
  return bounds
}

// dayStart is the first local instant of the day `offsetDays` after `day`. The Date
// constructor builds it in the local zone, so a DST change between days is absorbed.
function dayStart(day: string, offsetDays: number): string {
  const [y, m, d] = day.split('-').map(Number)
  return localRFC3339(new Date(y, m - 1, d + offsetDays))
}

function localRFC3339(t: Date): string {
  const pad = (n: number, width = 2) => String(n).padStart(width, '0')
  const offset = -t.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const abs = Math.abs(offset)
  return (
    `${pad(t.getFullYear(), 4)}-${pad(t.getMonth() + 1)}-${pad(t.getDate())}` +
    `T${pad(t.getHours())}:${pad(t.getMinutes())}:${pad(t.getSeconds())}` +
    `${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
  )
}
