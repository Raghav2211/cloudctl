// Package nlquery maps a natural-language request onto one of a small,
// hardcoded set of existing read-only cloudctl operations — the roadmap's
// Phase 4 "natural language cloud query" capability.
//
// The model (via ai.Client.Classify) only ever produces data — a JSON
// Intent — never a command, shell string, or AWS API call. Classify here
// then validates that Intent against a fixed whitelist before returning it;
// anything that doesn't match exactly is rejected with an explicit reason
// rather than guessed at. The caller (ctl ask) is responsible for actually
// dispatching a validated Intent to the corresponding existing executor —
// this package never executes anything itself.
package nlquery

import (
	"cloudctl/ai"
	"cloudctl/concept"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Intent is the fixed shape a classification must match before anything
// runs. Command/Subcommand/Flags select one of a small, hardcoded set of
// existing operations — see Vocabulary and validate below for the exact
// whitelist.
type Intent struct {
	Command           string            `json:"command"`
	Subcommand        string            `json:"subcommand"`
	Flags             map[string]string `json:"flags"`
	UnsupportedReason string            `json:"unsupported_reason"`
}

// Describe renders a validated Intent as a short, readable line — printed
// before dispatch so the interpreted action is never a silent surprise.
func (i *Intent) Describe() string {
	var b strings.Builder
	b.WriteString(i.Command)
	if i.Subcommand != "" {
		b.WriteString(" " + i.Subcommand)
	}
	keys := make([]string, 0, len(i.Flags))
	for k := range i.Flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, " --%s=%s", strings.ReplaceAll(k, "_", "-"), i.Flags[k])
	}
	return b.String()
}

// Vocabulary is the exact text given to the model describing what it may
// choose from. Kept next to validate below so the two can't silently drift
// apart — every flag/value validate accepts is described here, and nothing
// else.
const Vocabulary = `- command="find", subcommand="": a concept search across every discovered AWS resource type.
  flags.concept must be exactly one of: compute, container-cluster, database, function, network, storage
- command="ec2", subcommand="ls": list EC2 instances.
  flags.state (optional) must be one of: pending, running, shutting-down, terminated, stopping, stopped
  flags.az (optional): an availability zone, e.g. ap-south-1a
  flags.vpc (optional): a VPC ID
  flags.has_public_ip (optional): "true" or "false"
- command="changes", subcommand="": recent CloudTrail change timeline.
  flags.resource (optional): a resource name/ID to filter to
  flags.event (optional): a single CloudTrail event name to filter to
  flags.since (optional): how far back to look, a Go duration like "2h" or "30m" (default 2h)

"command" and "subcommand" are always separate JSON fields — never combine them into one string
like "ec2 ls". For example, EC2 must be encoded as {"command":"ec2","subcommand":"ls",...}.`

// Classify sends query to the model and validates the result against
// Vocabulary, returning a descriptive error for anything that doesn't
// cleanly resolve to exactly one supported intent — never a best-effort
// guess.
func Classify(ctx context.Context, client *ai.Client, query string) (*Intent, error) {
	raw, err := client.Classify(ctx, query, Vocabulary)
	if err != nil {
		return nil, fmt.Errorf("classifying query: %w", err)
	}

	jsonStr, ok := extractJSONObject(raw)
	if !ok {
		return nil, fmt.Errorf("query not supported: the model's response contained no JSON object (raw: %q)", raw)
	}

	var intent Intent
	if err := json.Unmarshal([]byte(jsonStr), &intent); err != nil {
		return nil, fmt.Errorf("query not supported: could not parse the model's response (%v) (raw: %q)", err, jsonStr)
	}
	if intent.Flags == nil {
		intent.Flags = map[string]string{}
	}
	normalizeCombinedCommand(&intent)

	if err := validate(&intent); err != nil {
		return nil, err
	}
	return &intent, nil
}

// extractJSONObject finds the first top-level {...} block in s — local
// "thinking" models routinely wrap JSON in chain-of-thought text or
// markdown fences despite being told to respond with only the object.
func extractJSONObject(s string) (string, bool) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return "", false
	}
	return s[start : end+1], true
}

// normalizeCombinedCommand tolerates a common local-model mistake despite
// Vocabulary explicitly instructing otherwise: writing "command":"ec2 ls"
// as one combined string instead of separate command/subcommand fields.
// This only ever splits an already-produced value — it never invents a
// command, so it can't widen what validate ultimately accepts.
func normalizeCombinedCommand(i *Intent) {
	if i.Subcommand != "" {
		return
	}
	command := strings.TrimSpace(i.Command)
	if idx := strings.IndexByte(command, ' '); idx > 0 {
		i.Command = command[:idx]
		i.Subcommand = strings.TrimSpace(command[idx+1:])
	}
}

func validate(i *Intent) error {
	if i.UnsupportedReason != "" {
		return fmt.Errorf("query not supported: %s", i.UnsupportedReason)
	}
	switch i.Command {
	case "find":
		return validateFind(i)
	case "ec2":
		return validateEC2(i)
	case "changes":
		return validateChanges(i)
	case "":
		return fmt.Errorf("query not supported: the model did not select a command")
	default:
		return fmt.Errorf("query not supported: %q is not a known command", i.Command)
	}
}

func validateFind(i *Intent) error {
	c := i.Flags["concept"]
	for _, known := range concept.All() {
		if string(known) == c {
			return nil
		}
	}
	return fmt.Errorf("query not supported: %q is not a known concept (try: compute, container-cluster, database, function, network, storage)", c)
}

var ec2LSAllowedFlags = map[string]bool{"state": true, "az": true, "vpc": true, "has_public_ip": true}
var ec2LSAllowedStates = map[string]bool{
	"pending": true, "running": true, "shutting-down": true,
	"terminated": true, "stopping": true, "stopped": true,
}

func validateEC2(i *Intent) error {
	if i.Subcommand != "ls" {
		return fmt.Errorf("query not supported: ec2 %q is not a known subcommand (only ls is supported)", i.Subcommand)
	}
	for k := range i.Flags {
		if !ec2LSAllowedFlags[k] {
			return fmt.Errorf("query not supported: %q is not a known ec2 ls flag", k)
		}
	}
	if state := i.Flags["state"]; state != "" && !ec2LSAllowedStates[state] {
		return fmt.Errorf("query not supported: %q is not a known instance state", state)
	}
	if v := i.Flags["has_public_ip"]; v != "" && v != "true" && v != "false" {
		return fmt.Errorf("query not supported: has_public_ip must be \"true\" or \"false\", got %q", v)
	}
	return nil
}

var changesAllowedFlags = map[string]bool{"resource": true, "event": true, "since": true}

func validateChanges(i *Intent) error {
	for k := range i.Flags {
		if !changesAllowedFlags[k] {
			return fmt.Errorf("query not supported: %q is not a known changes flag", k)
		}
	}
	if since := i.Flags["since"]; since != "" {
		if _, err := time.ParseDuration(since); err != nil {
			return fmt.Errorf("query not supported: %q is not a valid duration for since (e.g. \"2h\", \"30m\")", since)
		}
	}
	return nil
}
