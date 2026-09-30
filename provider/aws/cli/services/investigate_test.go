package services

import (
	"cloudctl/evidence"
	"cloudctl/investigate"
	"context"
	"path/filepath"
	"testing"

	"cloudctl/snapshot"
)

func TestPersistInvestigation_RoundTripsThroughInvestigationFromStored(t *testing.T) {
	t.Setenv("CLOUDCTL_SNAPSHOT_DB", filepath.Join(t.TempDir(), "snapshot.db"))
	ctx := context.Background()

	original := &investigate.Investigation{
		Question: "why is orders-db slow",
		Steps: []investigate.StepRecord{
			{Tool: "rds_stats", Args: map[string]string{"identifier": "orders-db"}, Reason: "check CPU", Summary: "CPU 84%"},
			{Tool: "rds_events", Args: map[string]string{"identifier": "orders-db"}, Err: "boom"},
		},
		Evidence: []evidence.Evidence{
			{Source: "cloudwatch:GetMetricStatistics", ResourceID: "orders-db", Field: "CPUPercent24h", Value: 84.0, Confidence: evidence.Fact},
		},
		StoppedReason:   "concluded",
		Conclusion:      "CPU is pegged",
		Summary:         "orders-db shows sustained high CPU",
		Recommendations: "scale up the instance class",
	}

	id, err := persistInvestigation(ctx, original)
	if err != nil {
		t.Fatalf("persisting investigation: %v", err)
	}
	if id == 0 {
		t.Fatal("expected a non-zero investigation ID")
	}

	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer store.Close()

	stored, err := store.GetInvestigation(ctx, id)
	if err != nil {
		t.Fatalf("getting stored investigation: %v", err)
	}

	got, err := investigationFromStored(stored)
	if err != nil {
		t.Fatalf("reconstructing investigation: %v", err)
	}

	if got.Question != original.Question || got.Conclusion != original.Conclusion || got.Summary != original.Summary {
		t.Errorf("expected top-level fields to round-trip, got %+v", got)
	}
	if len(got.Steps) != 2 || got.Steps[0].Tool != "rds_stats" || got.Steps[0].Args["identifier"] != "orders-db" {
		t.Fatalf("expected steps to round-trip, got %+v", got.Steps)
	}
	if got.Steps[1].Err != "boom" {
		t.Errorf("expected a failed step's error to round-trip, got %+v", got.Steps[1])
	}
	if len(got.Evidence) != 1 || got.Evidence[0].Field != "CPUPercent24h" || got.Evidence[0].Confidence != evidence.Fact {
		t.Fatalf("expected evidence to round-trip, got %+v", got.Evidence)
	}
}
