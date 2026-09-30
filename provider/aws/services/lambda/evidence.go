package lambda

import (
	"cloudctl/evidence"
	"cloudctl/health"
)

// derefStr returns "-" for a nil pointer instead of dereferencing it.
func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// functionDefinitionEvidence converts an already-fetched functionDefinition's
// configuration into Fact-tagged evidence for Summarize/Recommend, mirroring
// tableDefinitionEvidence (dynamodb) and instanceDefinitionEvidence (ec2).
func functionDefinitionEvidence(def *functionDefinition) []evidence.Evidence {
	if def == nil || def.name == nil {
		return nil
	}
	const source = "lambda:GetFunctionConfiguration"
	name := derefStr(def.name)

	facts := []evidence.Evidence{
		{Source: source, ResourceID: name, Field: "Runtime", Value: def.runtime, Confidence: evidence.Fact},
		{Source: source, ResourceID: name, Field: "State", Value: def.state, Confidence: evidence.Fact},
		{Source: source, ResourceID: name, Field: "PackageType", Value: def.packageType, Confidence: evidence.Fact},
		{Source: source, ResourceID: name, Field: "Handler", Value: derefStr(def.handler), Confidence: evidence.Fact},
	}
	if def.memorySizeMB != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: name, Field: "MemorySizeMB", Value: *def.memorySizeMB, Confidence: evidence.Fact})
	}
	if def.timeoutSec != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: name, Field: "TimeoutSeconds", Value: *def.timeoutSec, Confidence: evidence.Fact})
	}

	facts = append(facts, healthEvidence(name, functionHealth(def))...)

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
