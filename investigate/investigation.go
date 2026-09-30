package investigate

import "cloudctl/evidence"

// StepRecord is one executed step in an investigation's timeline: a
// validated tool call and what it returned (or the error it hit — a failed
// step is still recorded, never silently dropped).
type StepRecord struct {
	Tool    string
	Args    map[string]string
	Reason  string
	Summary string
	Err     string
}

// Investigation is the full record of one bounded investigation run: every
// step taken, all evidence gathered along the way, and the final AI
// synthesis over that evidence — the roadmap's Phase 7 "evidence-based
// investigation" aggregation object, produced naturally as a byproduct of
// Phase 6's loop rather than as separate work.
type Investigation struct {
	Question string
	Steps    []StepRecord
	Evidence []evidence.Evidence

	// StoppedReason explains why the loop ended: "concluded" (the model
	// chose to stop), "max_iterations" (the hard cap was hit), "model_stuck"
	// (three consecutive unparseable/invalid responses), or "ai_unavailable"
	// (the model backend itself could not be reached).
	StoppedReason string
	Conclusion    string

	Summary            string
	SummaryUnavailable string

	Recommendations            string
	RecommendationsUnavailable string
}
