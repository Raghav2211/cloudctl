// Package concept normalizes provider-specific resource types (e.g.
// "ec2:instance", "s3:bucket") into a small set of generic, provider-agnostic
// concepts (compute, storage, database, ...), so a user can find resources
// without knowing which cloud service or API they live behind. This package
// itself has no AWS (or any provider) import — it's a pure string mapping —
// which is what keeps it reusable if a second provider is added later.
package concept

// Concept is a generic resource category a user thinks in terms of,
// independent of any specific cloud provider or service.
type Concept string

const (
	Compute          Concept = "compute"
	ContainerCluster Concept = "container-cluster"
	Database         Concept = "database"
	Function         Concept = "function"
	Network          Concept = "network"
	Storage          Concept = "storage"
	Unknown          Concept = "unknown"
)

// resourceTypeConcepts maps a provider's "<service>:<kind>" resource type
// string (the same strings snapshot.Resource.Type already uses) to the
// Concept it belongs to. Every type a Discover function persists should have
// an entry here.
var resourceTypeConcepts = map[string]Concept{
	"ec2:instance":    Compute,
	"lambda:function": Function,
	"s3:bucket":       Storage,
	"dynamodb:table":  Database,
	"rds:instance":    Database,
	"rds:cluster":     Database,
	"eks:cluster":     ContainerCluster,
	"vpc:vpc":         Network,
}

// Of returns the Concept a resource type belongs to, or Unknown if this
// package has no mapping for it yet.
func Of(resourceType string) Concept {
	if c, ok := resourceTypeConcepts[resourceType]; ok {
		return c
	}
	return Unknown
}

// ResourceTypes returns every resource type string that maps to c, so a
// cross-service query can enumerate exactly what to look up.
func ResourceTypes(c Concept) []string {
	var types []string
	for t, tc := range resourceTypeConcepts {
		if tc == c {
			types = append(types, t)
		}
	}
	return types
}

// All returns every known Concept except Unknown, in a fixed, stable order —
// used to validate CLI input and drive shell-completion suggestions.
func All() []Concept {
	return []Concept{Compute, ContainerCluster, Database, Function, Network, Storage}
}
