# ADR-002: Fix the stats data race with errgroup + semaphore, not a mutex

## Status
Accepted

## Context
`statisticsFetcher.Fetch()` (ec2/fetcher.go) launches one goroutine per running instance and
appends each result to a shared slice with no synchronization and no concurrency cap — a
confirmed data race plus unbounded concurrent CloudWatch API calls.

## Decision
Use `errgroup.WithContext` with a bounded semaphore channel, and index-write into a
pre-sized slice instead of appending from goroutines. Do not simply wrap the existing
`append` in a `sync.Mutex`.

## Consequences
- Solves both problems (the race and the unbounded fan-out) in one change instead of two.
- Adds `golang.org/x/sync/errgroup` as a dependency — acceptable; it's stdlib-adjacent
  (`golang.org/x`), not a third-party framework.
- Gets cancellation propagation for free via `errgroup.WithContext`.
- The semaphore cap is a judgment call (start conservative, e.g. 8 concurrent calls) to stay
  well under CloudWatch's default per-account TPS limits.
