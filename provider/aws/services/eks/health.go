package eks

import "cloudctl/health"

// clusterHealth derives a deterministic health rollup from the cluster's
// already-fetched status, degraded further if any node group isn't ACTIVE —
// never a new AWS call, never an AI judgment.
func clusterHealth(def *clusterDefinition) health.Assessment {
	if def == nil || def.status == nil {
		return health.Of(health.Unknown, "cluster status unavailable")
	}

	var status health.Status
	var reasons []string
	switch *def.status {
	case "ACTIVE":
		status = health.Healthy
	case "CREATING", "UPDATING":
		status = health.Degraded
		reasons = append(reasons, "cluster is "+*def.status)
	case "DELETING", "FAILED":
		status = health.Unhealthy
		reasons = append(reasons, "cluster status: "+*def.status)
	default:
		status = health.Unknown
		reasons = append(reasons, "unrecognized status: "+*def.status)
	}

	for _, ng := range def.nodeGroups {
		if ng.status == nil || *ng.status == "ACTIVE" {
			continue
		}
		reason := "node group " + derefStr(ng.name) + " is " + derefStr(ng.status)
		reasons = append(reasons, reason)
		if status == health.Healthy {
			status = health.Degraded
		}
	}

	return health.Assessment{Status: status, Reasons: reasons}
}
