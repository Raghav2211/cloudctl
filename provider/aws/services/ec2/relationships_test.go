package ec2

import (
	"cloudctl/snapshot"
	"context"
	"path/filepath"
	"testing"
)

// isolateSnapshotStore points CLOUDCTL_SNAPSHOT_DB at a fresh temp file so
// tests never read or write the real user's ~/.cloudctl/snapshot.db.
func isolateSnapshotStore(t *testing.T) {
	t.Helper()
	t.Setenv("CLOUDCTL_SNAPSHOT_DB", filepath.Join(t.TempDir(), "snapshot.db"))
}

func TestSaveRelationships_WritesOneEdgePerDistinctSecurityGroup(t *testing.T) {
	isolateSnapshotStore(t)
	ctx := context.Background()

	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		t.Fatalf("failed to open isolated snapshot store: %v", err)
	}
	snapshotID, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		t.Fatalf("failed to seed a snapshot: %v", err)
	}
	store.Close()

	sg1, sg2 := "sg-111", "sg-222"
	def := newInstanceDefinition()
	def.SetInstanceIngressEgressRuleSummary(&instanceIngressEgressRuleSummary{
		ingressRules: []*ingressRule{{sgId: &sg1}, {sgId: &sg2}},
		egressRules:  []*egressRule{{sgId: &sg1}}, // same SG as an ingress rule: must not double-count
	})

	def.saveRelationships(ctx, "i-abc123")

	if def.relationshipsError != "" {
		t.Fatalf("expected relationships to persist cleanly, got error: %s", def.relationshipsError)
	}
	if def.relationshipsSaved != 2 {
		t.Errorf("expected 2 distinct security-group edges saved, got %d", def.relationshipsSaved)
	}

	store, err = snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		t.Fatalf("reopening store: %v", err)
	}
	defer store.Close()
	rels, err := store.ListRelationshipsFrom(ctx, snapshotID, "i-abc123")
	if err != nil {
		t.Fatalf("listing relationships: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("expected 2 persisted relationships, got %d", len(rels))
	}
	for _, r := range rels {
		if r.Kind != "ec2:security-group" {
			t.Errorf("expected kind ec2:security-group, got %q", r.Kind)
		}
	}
}

func TestSaveRelationships_NoSecurityGroupsIsNoop(t *testing.T) {
	isolateSnapshotStore(t)

	def := newInstanceDefinition()
	def.saveRelationships(context.Background(), "i-abc123")

	if def.relationshipsError != "" {
		t.Errorf("expected no error when there's nothing to relate, got %q", def.relationshipsError)
	}
	if def.relationshipsSaved != 0 {
		t.Errorf("expected 0 relationships saved, got %d", def.relationshipsSaved)
	}
}

// TestSaveRelationships_NoSnapshotYet_GracefulDegradation confirms that a
// security-group reference found before anyone has ever run `ctl discover
// aws` (so no snapshot exists to write into) degrades to a recorded error
// rather than panicking or blocking the rest of the command.
func TestLoadRelatedResources_BothDirections(t *testing.T) {
	isolateSnapshotStore(t)
	ctx := context.Background()

	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		t.Fatalf("failed to open isolated snapshot store: %v", err)
	}
	snapshotID, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		t.Fatalf("failed to seed a snapshot: %v", err)
	}
	// i-abc123 references sg-111 (From), and is itself referenced by
	// role/other via some unrelated relationship kind (To) — a stand-in for
	// a future writer, not something this package produces today.
	if err := store.SaveRelationship(ctx, snapshotID, "i-abc123", "sg-111", "ec2:security-group", nil); err != nil {
		t.Fatalf("seeding relationship: %v", err)
	}
	if err := store.SaveRelationship(ctx, snapshotID, "role/other", "i-abc123", "some:other-kind", nil); err != nil {
		t.Fatalf("seeding relationship: %v", err)
	}
	store.Close()

	def := newInstanceDefinition()
	def.loadRelatedResources(ctx, "i-abc123")

	if len(def.relatedResources) != 2 {
		t.Fatalf("expected 2 related resources, got %d: %+v", len(def.relatedResources), def.relatedResources)
	}
	var sawFrom, sawTo bool
	for _, r := range def.relatedResources {
		if r.direction == "depends on" && r.id == "sg-111" {
			sawFrom = true
		}
		if r.direction == "referenced by" && r.id == "role/other" {
			sawTo = true
		}
	}
	if !sawFrom || !sawTo {
		t.Errorf("expected both directions represented, got %+v", def.relatedResources)
	}
}

func TestLoadRelatedResources_NoSnapshotYetLeavesEmpty(t *testing.T) {
	isolateSnapshotStore(t)

	def := newInstanceDefinition()
	def.loadRelatedResources(context.Background(), "i-abc123")

	if len(def.relatedResources) != 0 {
		t.Errorf("expected no related resources with no snapshot to query, got %+v", def.relatedResources)
	}
}

func TestSaveRelationships_NoSnapshotYet_GracefulDegradation(t *testing.T) {
	isolateSnapshotStore(t) // fresh store, migrated schema, but zero snapshots

	sg1 := "sg-111"
	def := newInstanceDefinition()
	def.SetInstanceIngressEgressRuleSummary(&instanceIngressEgressRuleSummary{
		ingressRules: []*ingressRule{{sgId: &sg1}},
	})

	def.saveRelationships(context.Background(), "i-abc123")

	if def.relationshipsError == "" {
		t.Error("expected relationshipsError to explain that no snapshot exists yet")
	}
	if def.relationshipsSaved != 0 {
		t.Errorf("expected 0 relationships saved with no snapshot to write into, got %d", def.relationshipsSaved)
	}
}
