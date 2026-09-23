// Package snapshot provides a local SQLite store for discovered
// infrastructure resources, so commands can run against a point-in-time
// snapshot instead of always hitting live APIs. Per ADR-008, this is
// intentionally plain SQL (resources/relationships tables), not a graph
// database — nothing in this codebase's query patterns needs more.
package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Resource is a discovered resource ready to persist. Attrs is marshaled to
// JSON as-is, preserving full provider-specific detail rather than a
// hand-picked subset of fields.
type Resource struct {
	ID    string
	Type  string
	Attrs any
}

// StoredResource is a resource read back from the store. AttrsJSON is left
// for the caller to unmarshal into whatever concrete type it expects —
// the store itself has no opinion on resource shapes.
type StoredResource struct {
	ID        string
	Type      string
	AttrsJSON []byte
}

type Store struct {
	db *sql.DB
}

// DefaultPath returns ~/.cloudctl/snapshot.db, or the value of
// CLOUDCTL_SNAPSHOT_DB if set (used by tests, and by anyone who wants the
// store somewhere else).
func DefaultPath() string {
	if p := os.Getenv("CLOUDCTL_SNAPSHOT_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cloudctl", "snapshot.db")
}

// Open creates (if needed) and opens the snapshot store at path, creating
// its schema if this is a fresh database.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating snapshot store directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening snapshot store at %s: %w", path, err)
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	provider TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS resources (
	snapshot_id INTEGER NOT NULL REFERENCES snapshots(id),
	id TEXT NOT NULL,
	provider TEXT NOT NULL,
	type TEXT NOT NULL,
	attrs_json TEXT NOT NULL,
	PRIMARY KEY (snapshot_id, id)
);
CREATE TABLE IF NOT EXISTS relationships (
	snapshot_id INTEGER NOT NULL REFERENCES snapshots(id),
	source_id TEXT NOT NULL,
	target_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	evidence_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_resources_snapshot_type ON resources(snapshot_id, type);
CREATE INDEX IF NOT EXISTS idx_relationships_snapshot_source ON relationships(snapshot_id, source_id);
`)
	if err != nil {
		return fmt.Errorf("migrating snapshot store schema: %w", err)
	}
	return nil
}

// NewSnapshot records a new snapshot for provider (e.g. "aws") and returns
// its ID, to be passed to SaveResource for every resource discovered in
// this run.
func (s *Store) NewSnapshot(ctx context.Context, provider string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO snapshots (provider, created_at) VALUES (?, ?)`,
		provider, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("creating snapshot: %w", err)
	}
	return res.LastInsertId()
}

// SaveResource persists a discovered resource under the given snapshot.
func (s *Store) SaveResource(ctx context.Context, snapshotID int64, provider string, r Resource) error {
	attrsJSON, err := json.Marshal(r.Attrs)
	if err != nil {
		return fmt.Errorf("encoding resource %s: %w", r.ID, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO resources (snapshot_id, id, provider, type, attrs_json) VALUES (?, ?, ?, ?, ?)`,
		snapshotID, r.ID, provider, r.Type, string(attrsJSON))
	if err != nil {
		return fmt.Errorf("saving resource %s: %w", r.ID, err)
	}
	return nil
}

// LatestSnapshotID returns the most recent snapshot ID for provider, or an
// error explaining that no snapshot exists yet (and what command to run).
func (s *Store) LatestSnapshotID(ctx context.Context, provider string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM snapshots WHERE provider = ? ORDER BY id DESC LIMIT 1`, provider,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("no snapshot found for provider %q — run `ctl discover %s` first", provider, provider)
	}
	if err != nil {
		return 0, fmt.Errorf("finding latest snapshot for provider %q: %w", provider, err)
	}
	return id, nil
}

// SaveRelationship persists a discovered edge between two resources under
// the given snapshot (e.g. an IAM principal's cross-referenced access to an
// S3 bucket). evidenceJSON is the raw JSON that grounds the edge and may be
// nil if there's nothing more specific to record than the edge itself.
func (s *Store) SaveRelationship(ctx context.Context, snapshotID int64, sourceID, targetID, kind string, evidenceJSON []byte) error {
	var ej any
	if evidenceJSON != nil {
		ej = string(evidenceJSON)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO relationships (snapshot_id, source_id, target_id, kind, evidence_json) VALUES (?, ?, ?, ?, ?)`,
		snapshotID, sourceID, targetID, kind, ej)
	if err != nil {
		return fmt.Errorf("saving relationship %s -> %s (%s): %w", sourceID, targetID, kind, err)
	}
	return nil
}

// ListResources returns every resource of resourceType stored under
// snapshotID.
func (s *Store) ListResources(ctx context.Context, snapshotID int64, resourceType string) ([]StoredResource, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, attrs_json FROM resources WHERE snapshot_id = ? AND type = ?`,
		snapshotID, resourceType)
	if err != nil {
		return nil, fmt.Errorf("listing resources of type %q: %w", resourceType, err)
	}
	defer rows.Close()

	var out []StoredResource
	for rows.Next() {
		var r StoredResource
		var attrsJSON string
		if err := rows.Scan(&r.ID, &r.Type, &attrsJSON); err != nil {
			return nil, fmt.Errorf("scanning resource row: %w", err)
		}
		r.AttrsJSON = []byte(attrsJSON)
		out = append(out, r)
	}
	return out, rows.Err()
}
