// Typed access to the fitness-log endpoints — the dated training journal. Types
// mirror the Go API's JSON wire contract; the DB owns id, timestamps, and the
// entry_date default.
import { http } from './client'
import { setWindow, type TimeWindow } from './window'

// LogEntry is one dated journal entry: a markdown body with free-form tags and an
// optional mood. The id is a UUID7 string. mood is null when unset.
export interface LogEntry {
  id: string
  entry_date: string
  body: string
  tags: string[]
  mood: string | null
  created_at: string
  updated_at: string
}

// LogEntryCreate writes a new entry. entry_date defaults to today server-side when
// omitted; mood null (or omitted) is no mood.
export interface LogEntryCreate {
  entry_date?: string
  body: string
  tags?: string[]
  mood?: string | null
}

// LogEntryUpdate is a partial update: an omitted field is left unchanged. tags is
// replaced wholesale when supplied (an empty array clears them).
export interface LogEntryUpdate {
  entry_date?: string
  body?: string
  tags?: string[]
  mood?: string | null
}

export interface LogFilter extends TimeWindow {
  tag?: string
}

function queryString(filter: LogFilter): string {
  const params = new URLSearchParams()
  setWindow(params, filter)
  if (filter.tag) params.set('tag', filter.tag)
  const q = params.toString()
  return q ? `?${q}` : ''
}

export const logApi = {
  list: (filter: LogFilter = {}) => http.get<LogEntry[]>(`/log${queryString(filter)}`),
  get: (id: string) => http.get<LogEntry>(`/log/${encodeURIComponent(id)}`),
  create: (body: LogEntryCreate) => http.post<LogEntry>('/log', body),
  update: (id: string, body: LogEntryUpdate) => http.put<LogEntry>(`/log/${encodeURIComponent(id)}`, body),
  remove: (id: string) => http.del(`/log/${encodeURIComponent(id)}`),
}
