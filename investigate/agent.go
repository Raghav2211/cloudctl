package investigate

import (
	"cloudctl/ai"
	"cloudctl/viewer"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MaxIterations bounds how many tool calls a single investigation may make
// before being forced to conclude — a runaway-loop backstop, and a hard
// ceiling on how much local-model latency one `ctl investigate` call can
// cost, regardless of what the model decides to do.
const MaxIterations = 6

// maxConsecutiveInvalid stops the loop if the model can't produce a valid,
// whitelisted action several turns in a row — a sign it's stuck, not a
// transient slip worth retrying indefinitely.
const maxConsecutiveInvalid = 3

// action is the model's raw per-turn decision, parsed from JSON. Never
// trusted directly — Run validates Tool against the registry and Args
// against the tool's Required list before anything executes.
type action struct {
	Step       string            `json:"step"` // "call_tool" | "conclude"
	Tool       string            `json:"tool"`
	Args       map[string]string `json:"args"`
	Reason     string            `json:"reason"`
	Conclusion string            `json:"conclusion"`
}

// Run executes a bounded ReAct-style investigation loop: each turn the
// model sees the question, the tool registry, and the history of steps
// taken so far, and must respond with exactly one action — call a specific
// whitelisted tool, or conclude. The model never runs anything itself; Run
// validates every action against registry before executing it, exactly
// like nlquery.Classify's whitelist discipline. The loop stops when the
// model concludes, when MaxIterations is reached, or when it fails to
// produce a valid action maxConsecutiveInvalid turns in a row.
func Run(ctx context.Context, client *ai.Client, registry *Registry, question string) *Investigation {
	inv := &Investigation{Question: question}
	vocabulary := registry.Describe()

	var history strings.Builder
	consecutiveInvalid := 0

	for i := 0; i < MaxIterations; i++ {
		raw, err := client.Act(ctx, question, vocabulary, history.String())
		if err != nil {
			inv.StoppedReason = "ai_unavailable"
			break
		}

		act, err := parseAction(raw)
		if err != nil {
			consecutiveInvalid++
			fmt.Fprintf(&history, "- step %d: invalid response from the model (%v), reminded of the required JSON format\n", i+1, err)
			if consecutiveInvalid >= maxConsecutiveInvalid {
				inv.StoppedReason = "model_stuck"
				break
			}
			continue
		}
		consecutiveInvalid = 0

		if act.Step == "conclude" {
			inv.StoppedReason = "concluded"
			inv.Conclusion = act.Conclusion
			break
		}

		tool, ok := registry.Lookup(act.Tool)
		if !ok {
			fmt.Fprintf(&history, "- step %d: requested unknown tool %q, rejected — choose only from the listed tools\n", i+1, act.Tool)
			continue
		}
		if missing := missingRequired(tool, act.Args); len(missing) > 0 {
			fmt.Fprintf(&history, "- step %d: called %s missing required args %v, rejected\n", i+1, act.Tool, missing)
			continue
		}

		result, runErr := tool.Run(ctx, act.Args)
		rec := StepRecord{Tool: act.Tool, Args: act.Args, Reason: act.Reason}
		if runErr != nil {
			rec.Err = runErr.Error()
			fmt.Fprintf(&history, "- step %d: called %s(%v) -> error: %v\n", i+1, act.Tool, act.Args, runErr)
		} else {
			rec.Summary = result.Summary
			inv.Evidence = append(inv.Evidence, result.Evidence...)
			fmt.Fprintf(&history, "- step %d: called %s(%v) -> %s\n", i+1, act.Tool, act.Args, result.Summary)
		}
		inv.Steps = append(inv.Steps, rec)

		if i == MaxIterations-1 {
			inv.StoppedReason = "max_iterations"
		}
	}
	if inv.StoppedReason == "" {
		inv.StoppedReason = "max_iterations"
	}

	inv.applyAINarration(ctx, client)
	return inv
}

func missingRequired(tool Tool, args map[string]string) []string {
	var missing []string
	for _, name := range tool.Required {
		if strings.TrimSpace(args[name]) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// parseAction extracts and validates the model's JSON action — local
// "thinking" models routinely wrap JSON in chain-of-thought text or
// markdown fences despite being told to respond with only the object.
func parseAction(raw string) (action, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return action{}, fmt.Errorf("no JSON object found in response")
	}

	var act action
	if err := json.Unmarshal([]byte(raw[start:end+1]), &act); err != nil {
		return action{}, fmt.Errorf("could not parse JSON: %w", err)
	}
	if act.Step != "call_tool" && act.Step != "conclude" {
		return action{}, fmt.Errorf("unknown step %q (must be \"call_tool\" or \"conclude\")", act.Step)
	}
	if act.Step == "call_tool" && strings.TrimSpace(act.Tool) == "" {
		return action{}, fmt.Errorf("call_tool step with no tool name")
	}
	return act, nil
}

// applyAINarration synthesizes the investigation's final summary and
// recommendations from all evidence gathered across every step, using the
// same additive, never-erroring narration pattern as every def/stats/
// events command in this codebase (ADR-0010).
func (inv *Investigation) applyAINarration(ctx context.Context, client *ai.Client) {
	if len(inv.Evidence) == 0 {
		inv.SummaryUnavailable = "no evidence could be gathered"
		inv.RecommendationsUnavailable = "no evidence could be gathered"
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, inv.Evidence); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, inv.Evidence); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})

	if result.summaryErr != "" {
		inv.SummaryUnavailable = result.summaryErr
	} else {
		inv.Summary = result.summary
	}
	if result.recommendedErr != "" {
		inv.RecommendationsUnavailable = result.recommendedErr
	} else {
		inv.Recommendations = result.recommendations
	}
}
