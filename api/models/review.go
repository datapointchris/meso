package models

// Review is the capstone read: recent training reality pulled into one structured
// payload so Claude can reason about the next mesocycle in-conversation and persist
// the draft with ordinary cycle writes. There is no server-side LLM — this endpoint
// only assembles the history; the reasoning happens wherever the CLI's JSON is read.
//
// Since is the window's start date ("2006-01-02") the server resolved from the
// requested duration, echoed back so the reader knows the span it is seeing.
// ActiveCycles gives the current plan as context ("what's next"); the three history
// slices are what actually got done, measured, and felt over the window.
type Review struct {
	Since        string            `json:"since"`
	ActiveCycles []Cycle           `json:"active_cycles"`
	Sessions     []WorkoutSession  `json:"sessions"`
	Measurements []Measurement     `json:"measurements"`
	LogEntries   []FitnessLogEntry `json:"log_entries"`
}
