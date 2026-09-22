# ADR-003: Delete the orphaned EC2 stats viewer

## Status
Accepted

## Context
`ec2/statistics_viewer.go` defines a fully-built `EC2StatisticsTableViewer` (~230 lines:
summary stats, health-status formatting, insights) that is never invoked. The live
`ec2StatisticsViewer` function in `ec2/viewer.go` builds its own table, populates it, then
discards it and returns an empty `viewer.NewTableViewer()` instead — dead code sitting next
to a live call site that silently discards its work.

## Decision
Delete `statistics_viewer.go` rather than wire it in. The fuller stats-table UX it implements
was written alongside the heuristic `viewer/table.go` bloat being removed in ADR-004 — likely
the same provenance, unreviewed and untested. If a richer stats table UX is wanted later, it
can be reintroduced deliberately against the cleaned-up `viewer` package, not resurrected
as-is.

## Consequences
- `ec2 stats` stops silently discarding its own output — `ec2StatisticsViewer` now returns
  the table it actually built.
- Removes ~230 lines of unreviewed, unused code.
- No functional regression: the deleted viewer was never reachable from any command.
