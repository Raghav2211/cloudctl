# ADR-008: SQLite for the snapshot store, not JSON/BoltDB/a graph database

## Status
Accepted

## Context
Zero persistence exists today; the first cross-time feature (`ctl discover aws` +
`--from-snapshot`, and eventually Track 2D's blast-radius edges) needs some form of storage.

## Decision
SQLite via `modernc.org/sqlite` (pure-Go, no cgo — matters for shipping a single static
binary), pinned at v1.36.0 (the newest release still compatible with this module's `go 1.22`
directive — newer releases require go 1.23/1.24, which would force an unrelated toolchain
bump just to add a storage dependency). Three tables, per §9 Stage 1 / §13:

```sql
CREATE TABLE snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE resources (
    snapshot_id INTEGER NOT NULL REFERENCES snapshots(id),
    id TEXT NOT NULL,
    provider TEXT NOT NULL,
    type TEXT NOT NULL,
    attrs_json TEXT NOT NULL,
    PRIMARY KEY (snapshot_id, id)
);
CREATE TABLE relationships (
    snapshot_id INTEGER NOT NULL REFERENCES snapshots(id),
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    evidence_json TEXT
);
```

`attrs_json` stores the full, unmodified AWS SDK type (`types.Instance`, `types.Bucket`) as
JSON — preserving full provider-specific detail per §10, rather than a hand-picked subset of
fields that would need updating every time a new command wants a different attribute.
`relationships` is created now but left unpopulated until Track 2D writes into it — this
session (1D) only needs `resources`.

Default DB location: `~/.cloudctl/snapshot.db`, overridable via `CLOUDCTL_SNAPSHOT_DB` for
tests and for anyone who wants it elsewhere.

## Consequences
- No new external dependency beyond the Go module itself — no server process, no separate
  install step, consistent with shipping a single static binary.
- `attrs_json`'s round-trip fidelity depends on the AWS SDK types marshaling cleanly through
  `encoding/json` (they do — plain structs, enum fields are just named strings, no custom
  codecs) — verified by reading `ctl ec2 ls --from-snapshot` data back through the *same*
  `newInstanceSummary` constructor the live path uses, so any round-trip data loss would show
  up immediately as a rendering difference, not require separate verification logic.
- Relationship queries ("what points to X") are trivial indexed lookups once Track 2D starts
  writing to `relationships` — no query pattern identified anywhere in this review needs more
  than 2-3 hop traversal, which plain SQL (including recursive CTEs if ever needed) handles
  fine at CLI scale. A dedicated graph database remains unjustified — see §9 Stage 2's
  conditions, neither of which is close to true for this tool.
