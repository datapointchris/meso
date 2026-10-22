package models

// ReviewSchemaVersion is the shape of the Review payload. A field reshaped or
// removed raises it; an added field does not, because a reader ignores what it
// does not know.
const ReviewSchemaVersion = 1

// Review is the capstone read: recent training reality pulled into one structured
// payload so Claude can reason about the next mesocycle in-conversation and persist
// the draft with ordinary cycle writes. There is no server-side LLM — this endpoint
// only assembles the history; the reasoning happens wherever the CLI's JSON is read.
//
// Since and Until are the first and last days ("2006-01-02") the window covers,
// echoed back so the reader knows the span it is seeing, and empty where the caller
// left that side open. ActiveCycles gives the current plan as context ("what's
// next"); the three history slices are what actually got done, measured, and felt
// over the window. SchemaVersion is ReviewSchemaVersion, so a reader built for an
// older shape can refuse the payload rather than half-read it.
type Review struct {
	SchemaVersion int               `json:"schema_version"`
	Since         string            `json:"since"`
	Until         string            `json:"until"`
	ActiveCycles  []Cycle           `json:"active_cycles"`
	Sessions      []WorkoutSession  `json:"sessions"`
	Measurements  []Measurement     `json:"measurements"`
	LogEntries    []FitnessLogEntry `json:"log_entries"`
}
