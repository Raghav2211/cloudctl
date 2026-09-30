package rds

import "testing"

func TestDBDefinitionEvidence_Instance(t *testing.T) {
	def := newDBDefinitionFromInstance(testDBInstance("orders-db"))
	facts := dbDefinitionEvidence(def)

	// Kind, Engine, EngineVersion, Status, MultiAZ, StorageEncrypted, PubliclyAccessible, InstanceClass, BackupRetentionDays, computed Health
	if len(facts) != 10 {
		t.Fatalf("expected 10 facts, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Field == "Health" {
			if f.Confidence != "INFERENCE" {
				t.Errorf("expected the computed Health fact to be Inference-tagged, got %q", f.Confidence)
			}
			continue
		}
		if f.Confidence != "FACT" {
			t.Errorf("expected all non-health evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
	}
}

func TestDBDefinitionEvidence_Cluster(t *testing.T) {
	def := newDBDefinitionFromCluster(testDBCluster("analytics-cluster"))
	facts := dbDefinitionEvidence(def)

	// Kind, Engine, EngineVersion, Status, MultiAZ, StorageEncrypted, PubliclyAccessible, computed Health — no InstanceClass/BackupRetentionDays for this fixture
	if len(facts) != 8 {
		t.Fatalf("expected 8 facts, got %d: %+v", len(facts), facts)
	}
}

func TestDBDefinitionEvidence_NilDefinition(t *testing.T) {
	if facts := dbDefinitionEvidence(nil); facts != nil {
		t.Fatalf("expected nil facts for a nil definition, got %+v", facts)
	}
}
