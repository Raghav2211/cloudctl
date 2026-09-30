# ADR-021: Persist investigations to the existing local snapshot store

## Status
Accepted

## Context
ADR-020 shipped `ctl investigate` as a one-shot terminal report: run it, read the output, it's
gone. The roadmap's Phase 7 (evidence-based investigation) asks for investigations to be a durable
audit trail, not a single ephemeral render — useful both for revisiting a past finding without
re-running the (slow, multi-Ollama-call) loop, and for building a history of what's been
investigated over time.

## Decision
Persist every completed `investigate.Investigation` to the same local SQLite store `ctl discover`
already uses (`snapshot.Store`, `~/.cloudctl/snapshot.db`), rather than introducing a second local
database or a new top-level package for this one table:

- A new `investigations` table (`snapshot/investigations.go`) stores each run's question, stopped
  reason, conclusion, AI summary/recommendations (and their `...Unavailable` reasons), plus the
  step timeline and evidence trail as JSON blobs — mirroring `StoredResource`'s existing convention
  of leaving JSON shape to the caller rather than the store package taking an opinion on it. This
  keeps `snapshot` free of any dependency on `investigate`/`evidence`; the CLI layer (which already
  imports both) does the marshal/unmarshal.
- `ctl investigate aws` is restructured from one bare command into three subcommands: `run
  "<question>"` (unchanged behavior, now also persists), `list` (newest-first, capped at
  `--limit`), and `show <id>` (replays a past investigation's exact output — no Ollama or AWS calls
  — by reconstructing an `*investigate.Investigation` from its stored JSON). This is a breaking
  change to the `run`-less invocation shipped in ADR-020, accepted because the command was merged
  in the same work session and has not yet been exercised against real infrastructure — there is no
  compatibility cost to correcting the shape now versus carrying an awkward "the bare command means
  run, but there are also list/show siblings" API forward.
- Saving is a best-effort side effect inside `investigationFetcher.Fetch`, after the investigation
  itself has already completed: a local store failure prints a warning to stderr and the
  investigation's own result still renders normally. The investigation the user is looking at right
  now must never be hidden behind a persistence failure — this mirrors ADR-010's graceful-
  degradation principle for AI narration, applied here to storage instead of inference.

## Consequences
- `ctl investigate aws show <id>` is a pure local read — useful for revisiting a finding
  instantly, and for sharing/piping a past investigation's `--output json` without needing live
  credentials or Ollama at all.
- The `investigations` table has no foreign key to `snapshots`/`resources` — an investigation isn't
  scoped to one discovery snapshot, since its tool calls hit AWS live (or, in future, other
  providers), not the local resource cache. `provider` is a plain column (e.g. `"aws"`), matching
  `snapshots.provider`'s existing convention, so a future second provider's investigations coexist
  in the same table without schema change.
- Every step (including rejected/failed ones) round-trips through JSON exactly as gathered — no
  summarization or truncation happens at persistence time, so `show` is always a faithful replay,
  never a lossy one.
