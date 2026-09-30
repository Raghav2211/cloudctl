package rds

import "cloudctl/health"

// dbHealth derives a deterministic health rollup from the instance/cluster's
// already-fetched status — never a new AWS call, never an AI judgment.
func dbHealth(def *dbDefinition) health.Assessment {
	if def == nil || def.status == nil {
		return health.Of(health.Unknown, "status unavailable")
	}
	switch *def.status {
	case "available":
		return health.Of(health.Healthy)
	case "backing-up", "modifying", "upgrading", "configuring-enhanced-monitoring",
		"configuring-log-exports", "starting", "stopping", "creating", "rebooting":
		return health.Of(health.Degraded, "status is "+*def.status)
	case "failed", "incompatible-restore", "incompatible-parameters",
		"storage-full", "incompatible-network":
		return health.Of(health.Unhealthy, "status: "+*def.status)
	case "stopped":
		return health.Of(health.Unknown, "stopped (may be intentional)")
	default:
		return health.Of(health.Unknown, "unrecognized status: "+*def.status)
	}
}
