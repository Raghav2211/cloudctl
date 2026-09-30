package dynamodb

import (
	"cloudctl/health"
	"strings"
)

// tableHealth derives a deterministic health rollup from the table's
// already-fetched status — never a new AWS call, never an AI judgment.
func tableHealth(def *tableDefinition) health.Assessment {
	if def == nil || def.status == nil {
		return health.Of(health.Unknown, "table status unavailable")
	}
	switch *def.status {
	case "ACTIVE":
		return health.Of(health.Healthy)
	case "CREATING", "UPDATING":
		return health.Of(health.Degraded, "table is "+strings.ToLower(*def.status))
	case "DELETING":
		return health.Of(health.Degraded, "table is being deleted")
	case "INACCESSIBLE_ENCRYPTION_CREDENTIALS", "ARCHIVING", "ARCHIVED":
		return health.Of(health.Unhealthy, "table status: "+*def.status)
	default:
		return health.Of(health.Unknown, "unrecognized status: "+*def.status)
	}
}
