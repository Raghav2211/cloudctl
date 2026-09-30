// Package investigate implements the roadmap's Phase 6 read-only
// investigation agent: a bounded loop that lets a local model choose which
// of a small, whitelisted set of existing read-only cloudctl operations to
// call next, given an investigation question and the evidence gathered so
// far, until it concludes or a hard iteration cap is reached.
//
// The model never runs anything itself. Every action it proposes is
// validated against Registry before Run executes it — the same whitelist
// discipline nlquery.Classify uses for single-command natural-language
// queries, extended here to a multi-step loop. Nothing outside the fixed
// tool set can ever run, regardless of what the model outputs.
package investigate

import (
	"cloudctl/evidence"
	"context"
)

// Tool is one whitelisted read-only operation the investigation agent may
// call. Run must not print anything itself — it returns Fact/Inference-
// tagged evidence (fed into the investigation's final synthesis) and a
// short plain-text summary (fed back into the agent's next-turn prompt, so
// the model can decide what to do next without re-reading the full
// evidence list every turn).
type Tool struct {
	Name        string
	Description string
	Required    []string
	Optional    []string
	Run         func(ctx context.Context, args map[string]string) (ToolResult, error)
}

// ToolResult is what a Tool.Run returns.
type ToolResult struct {
	Evidence []evidence.Evidence
	Summary  string
}
