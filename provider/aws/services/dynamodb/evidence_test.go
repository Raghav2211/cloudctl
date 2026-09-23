package dynamodb

import (
	"errors"
	"testing"
)

func TestTableDefinitionEvidence_FullyPopulated(t *testing.T) {
	def := newTableDefinition(testTableDescription())
	def.SetPITR("ENABLED")
	def.SetTTL("expiresAt", "ENABLED")

	facts := tableDefinitionEvidence(def)

	// Status, BillingMode, Encryption, GSI count, LSI count, Stream, PITR, TTL
	if len(facts) != 8 {
		t.Fatalf("expected 8 facts, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Confidence != "FACT" {
			t.Errorf("expected all evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
	}
}

// TestTableDefinitionEvidence_ExcludesFailedSubFetches mirrors the concern
// behind instanceDefinitionEvidence/bucketDefinitionEvidence: PITR and TTL
// are independent, separately-failable sub-fetches and must not contribute
// evidence when they failed.
func TestTableDefinitionEvidence_ExcludesFailedSubFetches(t *testing.T) {
	def := newTableDefinition(testTableDescription())
	def.SetPITRAPIError(errors.New("access denied"))
	def.SetTTLAPIError(errors.New("access denied"))

	facts := tableDefinitionEvidence(def)
	for _, f := range facts {
		if f.Field == "PointInTimeRecovery" || f.Field == "TimeToLive" {
			t.Fatalf("expected no evidence from a failed sub-fetch, got %+v", f)
		}
	}
}

func TestTableDefinitionEvidence_NilDefinition(t *testing.T) {
	if facts := tableDefinitionEvidence(nil); facts != nil {
		t.Fatalf("expected nil facts for a nil definition, got %+v", facts)
	}
}
