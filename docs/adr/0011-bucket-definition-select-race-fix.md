# ADR-011: Fix a select-on-closed-channel race in bucketConfigurationFetcher

## Status
Accepted

## Context
`ctl s3 def` crashed with a nil-pointer panic against a real bucket. Root cause: each of the
5 config dimensions (policy/versioning/tags/encryption/lifecycle) used two separate buffered
channels — one for data, one for error — both `defer close()`d by the producing goroutine
regardless of which one actually received a value. A receive on a closed, empty channel is
always immediately ready in Go, returning the zero value with no blocking. So by the time the
collecting `select` ran, *both* cases were simultaneously ready whenever a call succeeded: the
data channel (holding the real result) and the now-closed, empty error channel (ready to yield
`nil` instantly). Go's `select` picks pseudo-randomly among ready cases — so roughly half the
time per field, the nil-error branch won, `SetPolicyAPIError(nil)` (or the equivalent for
another field) was called, and neither that field's data nor its error was ever actually set.
`Pretty()` then dereferenced the still-nil `*policy`/`*version`/etc., panicking.

This bug predates this whole review — it was present in the original codebase and was
preserved unmodified through the Track 1B typed-pipeline rewrite (only `ctx` threading and
typed local captures were added around it) because `bucketConfigurationFetcher` was explicitly
scoped out of interface extraction and testing in ADR-007, so no test ever exercised a full
`Fetch()` call end-to-end.

## Decision
Replace the two-channel-per-dimension pattern with one channel per dimension carrying a small
`{data, err}` result struct, so there is exactly one value to receive per dimension — no
ambiguity for `select` to race on, and in fact no `select` needed at all (a plain blocking
receive suffices, since each channel is guaranteed exactly one send). This also removes the
now-unnecessary `sync.WaitGroup` and `sync.Mutex`: `definition`'s fields are only ever written
from the calling goroutine after each blocking receive, never from the worker goroutines
themselves, so there's no concurrent access to guard.

Extracted a `bucketConfigurationAPI` interface (the 5 `GetBucket*` methods) so this fix has a
real regression test — the same call this bug just broke in production, run 200 times against
a fake client, asserting no dimension is ever left in the `nil`/`nil` (no data, no error)
state. Verified empirically both ways: the test passes reliably against the fix, and fails on
its very first iteration when the old two-channel pattern is temporarily reintroduced.

## Consequences
- `ctl s3 def` no longer panics. This was a real, user-facing crash on real infrastructure,
  not a theoretical finding.
- `bucketConfigurationFetcher` now has a client-interface boundary and a real test, closing the
  ADR-007 gap that let this bug ship unnoticed.
- The fix is also simpler than what it replaced (fewer channels, no mutex, no waitgroup) —
  correctness and simplicity improved together here, not traded off.
