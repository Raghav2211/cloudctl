# ADR-020: Bounded, whitelisted ReAct loop for the read-only investigation agent

## Status
Accepted

## Context
The roadmap's Phase 6 asks for a "read-only investigation agent": given a natural-language
question, decide which of several existing read-only operations to run, adapt based on what each
one returns, and synthesize a final answer. This is qualitatively different from everything
`ai.Client` has done so far (`Summarize`, `Recommend`, `Classify`) — ADR-009 deliberately scoped
`ai.Client` to one narrow backend and explicitly deferred anything resembling a tool-calling
framework until it was an actual, funded need. Phase 6 is that need.

The risk this ADR has to manage: once a model's output can determine *what runs next*, across
*multiple* steps, the blast radius of a bad or malicious model response grows with every turn
compared to `nlquery.Classify`'s single-shot, single-command whitelist check.

## Decision
Add a new top-level `investigate` package (matching this repo's flat, no-`internal/` convention),
implementing a **bounded ReAct loop**, not a fixed single-shot plan:

- Each turn, `ai.Client.Act` (new method, same shape as `Classify`: build a prompt, return raw
  text, never parse/validate it itself) is given the question, the tool registry description, and
  a plain-text history of steps taken so far. It must respond with exactly one JSON action: call
  one named tool with args, or conclude.
- `investigate.Run` validates every action before executing anything — unknown tool name, missing
  required arg, or unparseable JSON all reject the turn and continue the loop with a note in the
  history, exactly like `nlquery.Classify`'s whitelist. The model is never trusted to gate its own
  output, and nothing outside `Registry` can ever run, regardless of what the model outputs.
- The loop is capped at `MaxIterations = 6` tool calls, and separately capped at 3 consecutive
  invalid/unparseable responses (`model_stuck`) — both are hard backstops against a runaway loop
  costing unbounded local-model latency, independent of what the model decides to do.
- Every whitelisted `Tool.Run` returns `([]evidence.Evidence, string, error)`: Fact/Inference-
  tagged evidence for the investigation's final synthesis, and a plain-text summary fed back into
  the next turn's prompt. The final synthesis reuses `ai.Client.Summarize`/`Recommend` over the
  full aggregated evidence trail — unmodified, no new AI capability needed for that half — which
  incidentally produces the roadmap's Phase 7 "evidence-based investigation" aggregation object
  (`Investigation{Steps, Evidence, Summary, Recommendations}`) as a natural byproduct of the loop,
  not separate work.

**Tool summaries reuse `--output json/yaml`'s existing machinery.** Rather than hand-writing a
summary renderer per tool, `viewer.StructuredJSON(v Viewer) string` (new) type-asserts to the
already-existing `Structurable` interface (built for Phase 0's `--output json/yaml`) and
JSON-encodes `Structured()`. Every `Investigate*` adapter function is therefore a thin, uniform
wrapper: run the same `Fetcher`/evidence-builder/`Viewer` a hand-typed command already uses, and
hand the agent the same structured shape `--output json` already produces. No new per-service
summary logic was written.

**Tool registry covers all 8 existing services** (ec2, rds, s3, eks, vpc, dynamodb, lambda,
changes), one `list` + one `def` at minimum, plus `stats`/`events`/`impact` where those commands
already exist. Deeper per-service tools (EC2 CloudWatch stats, security-group explain, Lambda log
search) are deliberately deferred — the registry is already ~19 tools, near the practical ceiling
for a local 7B model to reliably pick from without hallucinating a name or args shape.

**New entry point**: `ctl investigate aws <question>`, distinct from `ctl ask`. `ask` stays scoped
to Phase 4's "map this sentence onto one existing command" — fast, single-shot. `investigate` is
explicitly multi-step and slower (up to 6 tool calls, each of which may itself trigger its own
`def`/`stats`/`events` command's existing AI narration — see Consequences), so a separate command
sets the right expectation rather than blurring a fast lookup with a slow investigation under one
UX.

## Consequences
- Every tool call still goes through the underlying command's own `Fetch()`, which for several
  commands (`def`, `stats`, `events`, `changes`) already triggers its own `Summarize`/`Recommend`
  call per ADR-010. This means a single investigation can trigger well over a dozen Ollama round
  trips (per-tool narration + the agent's own `Act` calls + final synthesis), which is slow on
  typical local hardware. This is an accepted tradeoff for v1 — the alternative (a parallel
  narration-free fetch path per service) roughly doubles the surface area this feature touches for
  a latency optimization, not a correctness one. A follow-up could add an "explain: false" fetch
  mode if this proves painful in practice.
- The `Investigate*` adapter functions in each service package are thin wiring (fetch existing
  data, call the existing evidence builder, reuse the existing viewer for a JSON summary) and are
  not independently unit-tested, consistent with this codebase's existing convention that
  `New*CommandExecutor` constructors themselves aren't unit-tested either — only the `Fetcher`/
  evidence functions they wire together are.
- `investigate.Investigation` and `investigate.Viewer` are provider-agnostic (no AWS import), so a
  future second provider can reuse the whole loop by supplying its own `Registry` — the same
  multi-cloud-readiness posture as `executor.Fetcher[T]`/`viewer.Viewer`.
- A step that fails (tool error) or gets rejected (unknown tool, missing arg) is recorded in the
  investigation's history and shown to the model on the next turn, never silently dropped — the
  agent can see its own mistakes and try something else, and a human reviewing the transcript can
  audit exactly what happened and why, never a silent black box.
