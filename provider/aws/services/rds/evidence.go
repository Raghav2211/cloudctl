package rds

import "cloudctl/evidence"

// derefStr returns "-" for a nil pointer instead of dereferencing it.
func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func derefBool(b *bool) bool {
	return b != nil && *b
}

// dbDefinitionEvidence converts an already-fetched dbDefinition's
// configuration into Fact-tagged evidence for Summarize, mirroring every
// other *DefinitionEvidence builder in this codebase. Public accessibility
// and encryption are surfaced explicitly — both are the fields most likely
// to matter for a quick risk read of a database.
func dbDefinitionEvidence(def *dbDefinition) []evidence.Evidence {
	if def == nil || def.identifier == nil {
		return nil
	}
	source := "rds:Describe" + map[string]string{"instance": "DBInstances", "cluster": "DBClusters"}[def.kind]
	id := derefStr(def.identifier)

	facts := []evidence.Evidence{
		{Source: source, ResourceID: id, Field: "Kind", Value: def.kind, Confidence: evidence.Fact},
		{Source: source, ResourceID: id, Field: "Engine", Value: derefStr(def.engine), Confidence: evidence.Fact},
		{Source: source, ResourceID: id, Field: "EngineVersion", Value: derefStr(def.engineVersion), Confidence: evidence.Fact},
		{Source: source, ResourceID: id, Field: "Status", Value: derefStr(def.status), Confidence: evidence.Fact},
		{Source: source, ResourceID: id, Field: "MultiAZ", Value: derefBool(def.multiAZ), Confidence: evidence.Fact},
		{Source: source, ResourceID: id, Field: "StorageEncrypted", Value: derefBool(def.storageEncrypted), Confidence: evidence.Fact},
		{Source: source, ResourceID: id, Field: "PubliclyAccessible", Value: derefBool(def.publiclyAccessible), Confidence: evidence.Fact},
	}

	if def.instanceClass != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: id, Field: "InstanceClass", Value: derefStr(def.instanceClass), Confidence: evidence.Fact})
	}
	if def.backupRetentionDays != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: id, Field: "BackupRetentionDays", Value: *def.backupRetentionDays, Confidence: evidence.Fact})
	}

	return facts
}
