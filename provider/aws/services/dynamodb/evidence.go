package dynamodb

import (
	"cloudctl/evidence"
	"fmt"
)

// derefStr returns "-" for a nil pointer instead of dereferencing it.
func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// tableDefinitionEvidence converts an already-fetched tableDefinition's
// configuration into Fact-tagged evidence for Summarize, mirroring
// bucketDefinitionEvidence (s3) and instanceDefinitionEvidence (ec2). PITR
// and TTL are only included when their own fetch succeeded — a failed
// sub-fetch contributes no evidence rather than a misleading one.
func tableDefinitionEvidence(def *tableDefinition) []evidence.Evidence {
	if def == nil || def.name == nil {
		return nil
	}
	const source = "dynamodb:DescribeTable"
	tableName := derefStr(def.name)

	facts := []evidence.Evidence{
		{Source: source, ResourceID: tableName, Field: "Status", Value: derefStr(def.status), Confidence: evidence.Fact},
		{Source: source, ResourceID: tableName, Field: "BillingMode", Value: derefStr(def.billingMode), Confidence: evidence.Fact},
		{Source: source, ResourceID: tableName, Field: "Encryption", Value: derefStr(def.encryptionType), Confidence: evidence.Fact},
		{Source: source, ResourceID: tableName, Field: "GlobalSecondaryIndexCount", Value: def.gsiCount, Confidence: evidence.Fact},
		{Source: source, ResourceID: tableName, Field: "LocalSecondaryIndexCount", Value: def.lsiCount, Confidence: evidence.Fact},
	}

	if def.itemCount != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: tableName, Field: "ItemCount", Value: *def.itemCount, Confidence: evidence.Fact})
	}

	streamValue := "disabled"
	if def.streamEnabled {
		streamValue = fmt.Sprintf("enabled (%s)", derefStr(def.streamViewType))
	}
	facts = append(facts, evidence.Evidence{Source: source, ResourceID: tableName, Field: "Stream", Value: streamValue, Confidence: evidence.Fact})

	if def.pitrAPIErr == nil && def.pitrStatus != nil {
		facts = append(facts, evidence.Evidence{
			Source: "dynamodb:DescribeContinuousBackups", ResourceID: tableName, Field: "PointInTimeRecovery",
			Value: derefStr(def.pitrStatus), Confidence: evidence.Fact,
		})
	}
	if def.ttlAPIErr == nil && def.ttlStatus != nil {
		facts = append(facts, evidence.Evidence{
			Source: "dynamodb:DescribeTimeToLive", ResourceID: tableName, Field: "TimeToLive",
			Value: fmt.Sprintf("%s (attribute: %s)", derefStr(def.ttlStatus), derefStr(def.ttlAttribute)), Confidence: evidence.Fact,
		})
	}

	return facts
}
