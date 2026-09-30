package snapshot

import (
	"context"
	"testing"
)

func TestStore_SaveAndGetInvestigation_RoundTrips(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	id, err := store.SaveInvestigation(ctx, "aws", StoredInvestigation{
		Question:        "why is orders-db slow",
		StoppedReason:   "concluded",
		Conclusion:      "CPU is pegged",
		Summary:         "orders-db shows high CPU",
		Recommendations: "scale up the instance",
		StepsJSON:       []byte(`[{"Tool":"rds_stats","Args":{"identifier":"orders-db"}}]`),
		EvidenceJSON:    []byte(`[{"Source":"test","Field":"CPUPercent24h","Value":90}]`),
	})
	if err != nil {
		t.Fatalf("saving investigation: %v", err)
	}
	if id == 0 {
		t.Fatal("expected a non-zero investigation ID")
	}

	got, err := store.GetInvestigation(ctx, id)
	if err != nil {
		t.Fatalf("getting investigation: %v", err)
	}
	if got.Question != "why is orders-db slow" {
		t.Errorf("expected question to round-trip, got %q", got.Question)
	}
	if got.Conclusion != "CPU is pegged" {
		t.Errorf("expected conclusion to round-trip, got %q", got.Conclusion)
	}
	if string(got.StepsJSON) != `[{"Tool":"rds_stats","Args":{"identifier":"orders-db"}}]` {
		t.Errorf("expected steps JSON to round-trip exactly, got %q", got.StepsJSON)
	}
	if got.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestStore_GetInvestigation_NotFound(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.GetInvestigation(context.Background(), 999); err == nil {
		t.Fatal("expected an error for a nonexistent investigation ID, got nil")
	}
}

func TestStore_ListInvestigations_NewestFirstAndLimited(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, q := range []string{"first question", "second question", "third question"} {
		if _, err := store.SaveInvestigation(ctx, "aws", StoredInvestigation{
			Question: q, StoppedReason: "concluded", StepsJSON: []byte("[]"), EvidenceJSON: []byte("[]"),
		}); err != nil {
			t.Fatalf("saving investigation %q: %v", q, err)
		}
	}

	items, err := store.ListInvestigations(ctx, "aws", 2)
	if err != nil {
		t.Fatalf("listing investigations: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected limit=2 to cap the result at 2, got %d", len(items))
	}
	if items[0].Question != "third question" || items[1].Question != "second question" {
		t.Errorf("expected newest-first order, got %+v", items)
	}
}

func TestStore_ListInvestigations_FiltersByProvider(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.SaveInvestigation(ctx, "aws", StoredInvestigation{
		Question: "aws question", StoppedReason: "concluded", StepsJSON: []byte("[]"), EvidenceJSON: []byte("[]"),
	}); err != nil {
		t.Fatalf("saving aws investigation: %v", err)
	}
	if _, err := store.SaveInvestigation(ctx, "gcp", StoredInvestigation{
		Question: "gcp question", StoppedReason: "concluded", StepsJSON: []byte("[]"), EvidenceJSON: []byte("[]"),
	}); err != nil {
		t.Fatalf("saving gcp investigation: %v", err)
	}

	items, err := store.ListInvestigations(ctx, "aws", 10)
	if err != nil {
		t.Fatalf("listing investigations: %v", err)
	}
	if len(items) != 1 || items[0].Question != "aws question" {
		t.Fatalf("expected only the aws investigation, got %+v", items)
	}
}
