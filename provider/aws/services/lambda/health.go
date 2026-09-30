package lambda

import "cloudctl/health"

// functionHealth derives a deterministic health rollup from the function's
// already-fetched state — never a new AWS call, never an AI judgment.
func functionHealth(def *functionDefinition) health.Assessment {
	if def == nil || def.state == "" {
		return health.Of(health.Unknown, "state unavailable")
	}
	switch def.state {
	case "Active":
		return health.Of(health.Healthy)
	case "Pending":
		return health.Of(health.Degraded, "function is still being created/updated")
	case "Failed":
		return health.Of(health.Unhealthy, "function state: Failed")
	case "Inactive":
		return health.Of(health.Degraded, "function is inactive and may need reactivation before its next invoke")
	default:
		return health.Of(health.Unknown, "unrecognized state: "+def.state)
	}
}
