# ADR-009: Evidence/Confidence types + one narrow Summarize call, backed by local Ollama

## Status
Accepted

## Context
The repo already has ~80% of an evidence model: `aws.ErrorInfo{Err, Meta, ErrorType}` answers
an *operational* question ("how bad is this problem"), not an *epistemic* one ("how certain am
I this claim is true"). The product principle established across this whole review — AI must
never be the source of facts, only of narration/correlation/hypothesis over facts gathered by
deterministic code — needs a type-level home before any feature can wire an LLM call into a
command.

The user chose a local Ollama model as the LLM backend for this codebase (over Anthropic/
OpenAI), so no API key/external network dependency is required to run these features.

## Decision
Add a new `evidence` package (`Confidence` enum: `FACT`/`INFERENCE`/`CORRELATION`/
`HYPOTHESIS`/`RECOMMENDATION`, and `Evidence{Source, ResourceID, Field, Value, Confidence}`),
and a new `ai` package with an Ollama-backed `Client.Summarize(ctx, facts []evidence.Evidence)
(string, error)`. No generic `LLMProvider` interface — `ai.Client` talks to exactly one backend
(a local Ollama server), configured via `OLLAMA_HOST` (default `http://localhost:11434`) and
`OLLAMA_MODEL` (default `llama3.2`, override for whatever model is actually pulled locally).
Introduce a provider abstraction only if/when a second concrete backend is an actual, funded
need — not speculatively now.

Both packages are new top-level packages (matching this repo's existing flat, no-`internal/`
convention — `executor`, `viewer`, `time` are structured the same way), not nested under a
prior package, since by the time 2B and 2C both consume `evidence`/`ai` unmodified, a shared
package is justified per §20's stated trigger ("hold off on a whole new top-level package until
both services consume the Confidence extension identically").

`Summarize` calls Ollama's `/api/generate` endpoint (non-streaming) with a prompt built solely
from the evidence list passed in, explicitly instructed not to invent facts beyond what's
listed. It returns the raw model text as a plain `string` — callers (2B, 2C) are responsible
for treating that string as `Hypothesis`-grade evidence, never as a new `Fact`; `Summarize`
itself does not construct `Evidence` values or assign `Confidence` (see ADR-011 for where that
invariant becomes load-bearing, in the mixed-confidence 2D feature).

## Consequences
- No API key, no external network call, no cost per invocation — matches "keep AI optional"
  and makes local development/testing free and offline-capable once a model is pulled.
- Callers must handle `Summarize` returning an error (unreachable Ollama server, no model
  pulled, empty response) and degrade gracefully — this is enforced at the 2B/2C call sites,
  not inside `Summarize` itself, which only reports the failure.
- `Client.Summarize`'s HTTP call goes through an injectable base URL, so tests point it at an
  `httptest.Server` instead of a real Ollama instance — no network access required to run
  `go test`.
- Ties this codebase's AI story to whatever the user has pulled locally; documented via the
  `OLLAMA_MODEL` env var rather than hardcoded, since the right model is an environment/
  hardware choice, not a code choice.
