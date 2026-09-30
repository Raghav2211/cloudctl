// Package health provides a small, deterministic vocabulary for rolling up
// a resource's already-fetched status/state fields into one of a few
// coarse categories, each with concrete reasons. This is never an AI
// judgment call and never issues a new AWS API call of its own — it's pure
// derivation from data a def command already fetched.
package health

// Status is a coarse, deterministic rollup of a resource's operational
// state.
type Status string

const (
	// Healthy means the resource is in its normal operating state.
	Healthy Status = "healthy"
	// Degraded means the resource is in a real but transitional or
	// partially-impaired state (e.g. still starting, mid-update).
	Degraded Status = "degraded"
	// Unhealthy means the resource is in a state that likely needs
	// attention (e.g. failed, storage full).
	Unhealthy Status = "unhealthy"
	// Unknown means the available data doesn't clearly map to any of the
	// above — including intentionally-stopped resources, which aren't
	// "unhealthy" so much as simply not running.
	Unknown Status = "unknown"
)

// Assessment is a resource's health rollup plus the concrete reasons behind
// it — a bare status with no explanation isn't useful on its own.
type Assessment struct {
	Status  Status
	Reasons []string
}

// Of builds an Assessment. reasons is optional for Healthy (a clean state
// speaks for itself) but should be given for every other status.
func Of(status Status, reasons ...string) Assessment {
	return Assessment{Status: status, Reasons: reasons}
}
