# ADR-010: AI narration is additive, never a replacement, and must degrade gracefully

## Status
Accepted

## Context
Idea #11 (bucket definition narrator) replaces `bucketDefinition.Pretty()`'s raw `fmt.Println`
dump with an AI-generated summary. `bucketConfigurationFetcher.Fetch` already fetches
policy/versioning/tags/encryption/lifecycle concurrently and successfully — the only thing
missing is turning that data into something readable. If the LLM call fails or Ollama isn't
running, the command must not fail outright: the deterministic data was already successfully
fetched and is still valuable on its own.

## Decision
Render the AI summary *alongside* the existing raw data, never in place of it, and treat
`Summarize` failures as non-fatal: `bucketConfigurationFetcher.Fetch` always returns a valid
`*bucketDefinition` with `(nil, nil)` error semantics unaffected by whether the AI call
succeeded — a failed `Summarize` call only sets `aiSummaryUnavailable` with a human-readable
reason, never turns into the `Fetch` method's returned `error`. Evidence is built from the
concrete, already-typed SDK responses (`*s3.GetBucketPolicyOutput` etc.) captured locally in
`Fetch`, not from the type-erased `*interface{}` fields already stored on `bucketDefinition`
via `SetPolicy`/`SetVersion`/etc. — this sidesteps needing to fix that pre-existing `P2`
type-erasure issue (§21) just to build evidence, since the concrete values are already in hand
at the exact point they're fetched.

## Consequences
- Directly enforces "keep AI optional" — the tool stays fully useful with Ollama not running,
  not installed, or slow; `ctl s3 def` never blocks or fails because of it.
- Makes this session the honest test of whether AI narration is worth keeping (per the
  Recommended Direction, §19): the raw facts are always right there to compare the summary
  against, so a wrong or unhelpful summary is immediately visible, not hidden behind an
  AI-or-nothing output.
- Evidence is built only from the dimensions that actually succeeded — a partial fetch (e.g.
  policy retrieval denied but everything else fine) still produces a useful, partial summary
  rather than an all-or-nothing one, matching the existing per-field partial-success pattern
  `bucketConfigurationFetcher.Fetch` already uses for its raw output.
