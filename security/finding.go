// Package security provides a small, deterministic vocabulary for
// structured security findings — the roadmap's Phase 8 ask for
// Finding{severity, resource, evidence, remediation} objects, as opposed to
// free-text AI narration. Every Finding here is derived by plain Go rules
// over already-fetched data, never by a model: this package has no
// dependency on cloudctl/ai at all. AI may still narrate a Report
// afterwards (recommendations, prioritization) exactly like every other
// def/stats command, but it never gets to invent or contribute a Finding
// itself, matching evidence.Confidence's rule that only deterministic code
// may assert Fact/Inference.
package security

import "cloudctl/evidence"

// Severity is a coarse, deterministic ranking of how urgent a Finding is.
type Severity string

const (
	// Critical means the resource is very likely already exposed to
	// significant risk (e.g. a database or bucket policy open to the
	// internet) and should be treated as urgent.
	Critical Severity = "CRITICAL"
	// High means a serious, well-known misconfiguration that should be
	// fixed soon, even if not immediately catastrophic.
	High Severity = "HIGH"
	// Medium means a real gap against security best practice, but lower
	// urgency than High/Critical.
	Medium Severity = "MEDIUM"
	// Low means a minor hardening opportunity.
	Low Severity = "LOW"
)

// Finding is one deterministic security issue detected on a resource.
// Evidence grounds the finding in the already-fetched facts that triggered
// it, so a human (or a follow-up AI narration pass) can verify the rule's
// conclusion without re-fetching anything.
type Finding struct {
	// Rule is a short, stable identifier for the check that produced this
	// finding (e.g. "rds-publicly-accessible") — stable across releases so
	// findings can be tracked/suppressed by rule ID over time.
	Rule        string
	Severity    Severity
	ResourceID  string
	Description string
	Remediation string
	Evidence    []evidence.Evidence
}
