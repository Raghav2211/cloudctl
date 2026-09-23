package rds

import "testing"

func TestDBDefinitionEvidence_Instance(t *testing.T) {
	def := newDBDefinitionFromInstance(testDBInstance("orders-db"))
	facts := dbDefinitionEvidence(def)

	// Kind, Engine, EngineVersion, Status, MultiAZ, StorageEncrypted, PubliclyAccessible, InstanceClass, BackupRetentionDays
	if len(facts) != 9 {
		t.Fatalf("expected 9 facts, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Confidence != "FACT" {
			t.Errorf("expected all evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
	}
}

func TestDBDefinitionEvidence_Cluster(t *testing.T) {
	def := newDBDefinitionFromCluster(testDBCluster("analytics-cluster"))
	facts := dbDefinitionEvidence(def)

	// Kind, Engine, EngineVersion, Status, MultiAZ, StorageEncrypted, PubliclyAccessible — no InstanceClass/BackupRetentionDays for this fixture
	if len(facts) != 7 {
		t.Fatalf("expected 7 facts, got %d: %+v", len(facts), facts)
	}
}

func TestDBDefinitionEvidence_NilDefinition(t *testing.T) {
	if facts := dbDefinitionEvidence(nil); facts != nil {
		t.Fatalf("expected nil facts for a nil definition, got %+v", facts)
	}
}
