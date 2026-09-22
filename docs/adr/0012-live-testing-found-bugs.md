# ADR-012: Two more real bugs found by testing Track 1D live against a real account

## Status
Accepted

## Context
Verifying `ctl discover aws` / `ctl ec2 ls --from-snapshot` required actually running the
tool against a real AWS account (necessary to validate the round-trip claim in ADR-008 —
mocked tests alone can't prove that). Doing so surfaced two pre-existing bugs that had never
been exercised before, neither introduced by this review's changes:

**1. `ec2 ls` with zero flags always failed.** `InstanceListFilter.requestFilters()`
initialized `filters := []types.Filter{}` and returned that empty-but-non-nil slice when no
filter applied. AWS's `DescribeInstances` rejects an explicitly-empty `Filters` list with
`InvalidRequest: The request received was invalid.` — confirmed live: the same account
returned this error with zero flags, but succeeded immediately once a real filter (e.g.
`--state running`) was added. This also silently broke `ctl discover aws`'s EC2 portion,
which calls `fetchInstanceList` with an intentionally empty filter (discovery is unfiltered
by design).

**2. `ec2 ls` crashed on any instance with a `LaunchTime`.** Every EC2 executor constructor
called `time.GetTZ("")` instead of the actual `--tz` flag value — a pre-existing bug present
in the original codebase, not something Track 1B/1C introduced. `GetTZ` looked up the
identifier in a map and returned the zero value (`nil`) on a miss, including on `""`. This
went unnoticed through every earlier session because every earlier live test happened to hit
an account/region with zero instances (a legitimate but unrepresentative case) — the crash
only triggers once `newInstanceSummary` calls `tz.AdaptTimezone()` on a real, non-nil
`LaunchTime`, which requires at least one real instance to exist.

## Decision
- Fixed `requestFilters()` to return `nil` (declare `var filters []types.Filter`, never
  initialize an empty literal) when no filter applies.
- Fixed `GetTZ` to fall back to UTC on any unrecognized or empty identifier instead of
  returning `nil` — this is a defensive fix at the root cause, not just a patch at each call
  site, so no future caller can reintroduce this exact crash by forgetting to check for nil.
- Also fixed the underlying gap that made `GetTZ("")` reachable at all: threaded the real
  `--tz` CLI flag value through every EC2 executor constructor (`NewinstanceListCommandExecutor`,
  `NewInstanceListFromSnapshotCommandExecutor`, `NewInstanceDescribeCommandExecutor`,
  `NewEC2StatisticsDescribeCommandExecutor`), mirroring the `cli *global.CLIFlag` Run()
  parameter pattern already proven working for S3 commands. EC2 commands now actually respect
  `--tz`, which they never did before.

## Consequences
- `ctl ec2 ls` with no flags, `ctl discover aws`, and `ctl ec2 ls --from-snapshot` were all
  verified against a real account with 36 real running instances and 147 real S3 buckets:
  identical instance ID sets and identical rendered data between the live call and the
  snapshot-backed read — the actual round-trip claim ADR-008 makes, proven, not assumed.
- Regression tests added: `TestInstanceListFilter_RequestFilters_NilWhenNoOptions` (asserts
  `nil`, not just `len() == 0`, closing the exact gap that let this ship) and
  `TestGetTZ_NeverReturnsNil`/`TestGetTZ_UnknownIdentifierFallsBackToUTC` in the `time`
  package (which had zero tests before this).
- Both bugs were invisible to every prior test and every prior live check in this review
  because every prior live check happened to run against zero-instance scenarios. This is a
  reminder that "verified live" is only as strong as the data it's verified against — an
  empty account exercises a materially different code path than a populated one.
