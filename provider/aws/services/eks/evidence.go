package eks

import (
	"cloudctl/evidence"
	"cloudctl/health"
	"fmt"
	"strings"
)

// derefStr returns "-" for a nil pointer instead of dereferencing it.
func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// clusterDefinitionEvidence converts an already-fetched clusterDefinition's
// configuration into Fact-tagged evidence for Summarize, mirroring every
// other *DefinitionEvidence builder in this codebase. Endpoint access mode
// is surfaced explicitly — a publicly-accessible control-plane endpoint is
// the single field most likely to matter for a quick risk read of a
// cluster.
func clusterDefinitionEvidence(def *clusterDefinition) []evidence.Evidence {
	if def == nil || def.name == nil {
		return nil
	}
	const source = "eks:DescribeCluster"
	name := derefStr(def.name)

	endpointAccess := "private-only"
	switch {
	case def.endpointPublicAccess && def.endpointPrivateAccess:
		endpointAccess = "public and private"
	case def.endpointPublicAccess:
		endpointAccess = "public-only"
	}

	facts := []evidence.Evidence{
		{Source: source, ResourceID: name, Field: "Version", Value: derefStr(def.version), Confidence: evidence.Fact},
		{Source: source, ResourceID: name, Field: "Status", Value: derefStr(def.status), Confidence: evidence.Fact},
		{Source: source, ResourceID: name, Field: "EndpointAccess", Value: endpointAccess, Confidence: evidence.Fact},
	}

	if def.endpointPublicAccess && len(def.publicAccessCidrs) > 0 {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: name, Field: "PublicAccessCidrs", Value: strings.Join(def.publicAccessCidrs, ", "), Confidence: evidence.Fact})
	}

	loggingValue := "none enabled"
	if len(def.enabledClusterLogTypes) > 0 {
		loggingValue = strings.Join(def.enabledClusterLogTypes, ", ")
	}
	facts = append(facts, evidence.Evidence{Source: source, ResourceID: name, Field: "ClusterLogging", Value: loggingValue, Confidence: evidence.Fact})

	for _, ng := range def.nodeGroups {
		facts = append(facts, evidence.Evidence{
			Source: "eks:DescribeNodegroup", ResourceID: name, Field: "NodeGroup",
			Value: fmt.Sprintf("%s: %s, %s, instances=%v, desired=%d min=%d max=%d",
				derefStr(ng.name), derefStr(ng.status), ng.capacityType, ng.instanceTypes,
				derefInt32(ng.desiredSize), derefInt32(ng.minSize), derefInt32(ng.maxSize)),
			Confidence: evidence.Fact,
		})
	}

	facts = append(facts, healthEvidence(name, clusterHealth(def))...)

	return facts
}

// healthEvidence converts a deterministic health.Assessment into
// Inference-tagged evidence (it's derived via a fixed rule from Facts
// already in this list, never an AI judgment) so the AI summary/
// recommendations for this resource can reference its computed health.
func healthEvidence(resourceID string, h health.Assessment) []evidence.Evidence {
	facts := []evidence.Evidence{
		{Source: "computed:health", ResourceID: resourceID, Field: "Health", Value: string(h.Status), Confidence: evidence.Inference},
	}
	for _, reason := range h.Reasons {
		facts = append(facts, evidence.Evidence{Source: "computed:health", ResourceID: resourceID, Field: "HealthReason", Value: reason, Confidence: evidence.Inference})
	}
	return facts
}

func derefInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}
