// Package cost provides structured cost-analysis output for the roadmap's
// Phase 9 — the roadmap's "cost analysis" ask, produced two possible ways
// depending on what the active AWS identity can access:
//
//   - CostSummary: real dollar spend by service, from AWS Cost Explorer
//     (ce:GetCostAndUsage) — the authoritative source, when that permission
//     is available.
//   - Finding: a deterministic per-resource idle/waste signal, derived from
//     already-fetched utilization data (CPU, connections, invocations) —
//     used as a fallback when Cost Explorer access isn't available, so a
//     cost-relevant signal is still shown instead of nothing.
//
// Like cloudctl/security, this package has no dependency on cloudctl/ai:
// every Finding here is produced by plain Go rules over already-fetched
// facts, never by a model.
package cost

import "cloudctl/evidence"

// Finding is one deterministic cost-waste signal detected on a resource —
// not a dollar estimate (this package doesn't have pricing data), but a
// concrete "this looks idle" observation grounded in evidence.
type Finding struct {
	Rule           string
	ResourceID     string
	Description    string
	Recommendation string
	Evidence       []evidence.Evidence
}
