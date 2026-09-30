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

	// Status, BillingMode, Encryption, GSI count, LSI count, Stream, PITR, TTL, computed Health
	if len(facts) != 9 {
		t.Fatalf("expected 9 facts, got %d: %+v", len(facts), facts)
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

// TestTableDefinitionEvidence_IncludesHealthReason confirms a non-healthy
// status contributes its HealthReason too, not just the bare Health status
// — the whole point of feeding health into evidence is so the AI summary
// can explain *why*, not just state the status.
func TestTableDefinitionEvidence_IncludesHealthReason(t *testing.T) {
	creating := testTableDescription()
	creating.TableStatus = "CREATING"
	def := newTableDefinition(creating)

	facts := tableDefinitionEvidence(def)

	var sawHealth, sawReason bool
	for _, f := range facts {
		if f.Field == "Health" && f.Value == "degraded" {
			sawHealth = true
		}
		if f.Field == "HealthReason" {
			sawReason = true
		}
	}
	if !sawHealth {
		t.Error("expected a degraded Health fact for a CREATING table")
	}
	if !sawReason {
		t.Error("expected a HealthReason fact explaining the degraded status")
	}
}

func TestTableDefinitionEvidence_NilDefinition(t *testing.T) {
	if facts := tableDefinitionEvidence(nil); facts != nil {
		t.Fatalf("expected nil facts for a nil definition, got %+v", facts)
	}
}
