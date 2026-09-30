package vpc

import "cloudctl/health"

// vpcHealth derives a deterministic health rollup from the VPC's
// already-fetched state — never a new AWS call, never an AI judgment.
func vpcHealth(def *vpcDefinition) health.Assessment {
	if def == nil || def.state == nil {
		return health.Of(health.Unknown, "state unavailable")
	}
	switch *def.state {
	case "available":
		return health.Of(health.Healthy)
	case "pending":
		return health.Of(health.Degraded, "VPC is still being created")
	default:
		return health.Of(health.Unknown, "unrecognized state: "+*def.state)
	}
}
