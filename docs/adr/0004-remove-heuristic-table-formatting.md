# ADR-004: Remove heuristic column-type sniffing instead of fixing its edge cases

## Status
Accepted

## Context
`viewer/table.go`'s `analyzeColumnType` guesses column semantics (time/id/ip/status/number/
metric) from a 5-row sample of rendered string content via substring/prefix matching, then
applies emoji decoration and colored transformers based on the guess. Every call site
(`instanceListViewer`, `bucketListViewer`, etc.) already knows its column types statically at
construction time — this is guessing at information the caller already has, and it
misclassifies on edge cases (e.g. an S3 object key containing a dash and a colon reads as a
"time" column).

## Decision
Delete the heuristics (`analyzeColumnType`, `getIntelligentColumnConfigs`,
`getEmojiForTitle`/`getEmojiForHeader`, `generic*Transformer` functions) rather than patch
their misclassification bugs. If per-column formatting is wanted in the future, add an
explicit `[]table.ColumnConfig` parameter that callers populate themselves.

## Consequences
- Removes ~350+ lines of fragile, ungrounded complexity.
- Any future formatting need is solved with information the caller already has, not by
  re-deriving it from string content — cheaper and more correct.
- Table rendering becomes plain (no emoji, no heuristic coloring) until/unless explicit
  per-column config is added deliberately.
