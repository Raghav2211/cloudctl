package ec2

import "cloudctl/health"

// instanceHealth derives a deterministic health rollup from the instance's
// already-fetched state — never a new AWS call, never an AI judgment.
func instanceHealth(def *instanceDefinition) health.Assessment {
	if def == nil || def.summary == nil || def.summary.state == nil {
		return health.Of(health.Unknown, "instance state unavailable")
	}
	switch *def.summary.state {
	case "running":
		return health.Of(health.Healthy)
	case "pending":
		return health.Of(health.Degraded, "instance is still starting")
	case "stopping", "shutting-down":
		return health.Of(health.Degraded, "instance is stopping")
	case "stopped":
		return health.Of(health.Unknown, "instance is stopped (may be intentional)")
	case "terminated":
		return health.Of(health.Unknown, "instance is terminated")
	default:
		return health.Of(health.Unknown, "unrecognized state: "+*def.summary.state)
	}
}
