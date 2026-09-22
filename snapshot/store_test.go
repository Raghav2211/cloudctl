package snapshot

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "snapshot.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestStore_SaveAndListResources_RoundTrips(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	snapshotID, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	type fakeAttrs struct {
		Name  string
		State string
	}
	err = store.SaveResource(ctx, snapshotID, "aws", Resource{
		ID: "i-abc123", Type: "ec2:instance", Attrs: fakeAttrs{Name: "web-1", State: "running"},
	})
	if err != nil {
		t.Fatalf("saving resource: %v", err)
	}
	err = store.SaveResource(ctx, snapshotID, "aws", Resource{
		ID: "my-bucket", Type: "s3:bucket", Attrs: fakeAttrs{Name: "my-bucket"},
	})
	if err != nil {
		t.Fatalf("saving resource: %v", err)
	}

	instances, err := store.ListResources(ctx, snapshotID, "ec2:instance")
	if err != nil {
		t.Fatalf("listing instances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	var got fakeAttrs
	if err := json.Unmarshal(instances[0].AttrsJSON, &got); err != nil {
		t.Fatalf("decoding attrs: %v", err)
	}
	if got.Name != "web-1" || got.State != "running" {
		t.Errorf("round-tripped attrs don't match: %+v", got)
	}

	buckets, err := store.ListResources(ctx, snapshotID, "s3:bucket")
	if err != nil {
		t.Fatalf("listing buckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket (type filter must not leak across types), got %d", len(buckets))
	}
}

func TestStore_LatestSnapshotID_NoneExists(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.LatestSnapshotID(context.Background(), "aws"); err == nil {
		t.Fatal("expected an error when no snapshot exists yet, got nil")
	}
}

func TestStore_LatestSnapshotID_ReturnsMostRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		t.Fatalf("creating first snapshot: %v", err)
	}
	second, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		t.Fatalf("creating second snapshot: %v", err)
	}
	if first == second {
		t.Fatal("expected distinct snapshot IDs")
	}

	latest, err := store.LatestSnapshotID(ctx, "aws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != second {
		t.Errorf("expected latest snapshot to be %d, got %d", second, latest)
	}
}

func TestStore_SaveResource_ReplaceOnSameID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	snapshotID, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	err = store.SaveResource(ctx, snapshotID, "aws", Resource{ID: "i-1", Type: "ec2:instance", Attrs: map[string]string{"state": "pending"}})
	if err != nil {
		t.Fatalf("saving resource: %v", err)
	}
	err = store.SaveResource(ctx, snapshotID, "aws", Resource{ID: "i-1", Type: "ec2:instance", Attrs: map[string]string{"state": "running"}})
	if err != nil {
		t.Fatalf("saving resource: %v", err)
	}

	resources, err := store.ListResources(ctx, snapshotID, "ec2:instance")
	if err != nil {
		t.Fatalf("listing resources: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected saving the same resource ID twice to replace, not duplicate; got %d rows", len(resources))
	}
	var attrs map[string]string
	if err := json.Unmarshal(resources[0].AttrsJSON, &attrs); err != nil {
		t.Fatalf("decoding attrs: %v", err)
	}
	if attrs["state"] != "running" {
		t.Errorf("expected the second save to win, got state=%q", attrs["state"])
	}
}
