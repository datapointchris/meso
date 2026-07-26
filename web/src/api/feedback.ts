// Typed access to the feedback endpoint. The SPA only ever captures — reading and
// triaging feedback is the CLI's job (`meso admin feedback`), so there is no list or
// update here. Types mirror the Go API's JSON wire contract.
import { http } from './client'

export type FeedbackStatus = 'open' | 'done'

// Feedback is one captured papercut or idea about the app itself. There is
// deliberately no kind/category — the body says what it is. The viewport is null for
// feedback captured through the CLI.
export interface Feedback {
  id: string
  status: FeedbackStatus
  body: string
  context_path: string
  viewport_width: number | null
  viewport_height: number | null
  created_at: string
  updated_at: string
}

// FeedbackCreate captures feedback. context_path is the in-app route it was raised
// from and the viewport is the window size it was seen at; status defaults to "open"
// server-side.
export interface FeedbackCreate {
  body: string
  context_path?: string
  viewport_width?: number
  viewport_height?: number
}

export const feedbackApi = {
  capture: (body: FeedbackCreate) => http.post<Feedback>('/feedback', body),
}
