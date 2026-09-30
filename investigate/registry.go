package investigate

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is the fixed, whitelisted set of tools the investigation agent
// may call. Nothing outside this set can ever run, regardless of what the
// model outputs (see Run's per-turn validation).
type Registry struct {
	tools map[string]Tool
}

// NewRegistry builds a Registry from a fixed tool list. A duplicate Name
// silently overwrites the earlier entry — callers are expected to pass a
// hand-authored, non-overlapping tool list, not user/model input.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		r.tools[t.Name] = t
	}
	return r
}

// Lookup returns the named tool, or false if name isn't in the whitelist.
func (r *Registry) Lookup(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Describe renders the registry as prompt vocabulary text, sorted by name
// for a stable, reproducible prompt across runs.
func (r *Registry) Describe() string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		t := r.tools[name]
		fmt.Fprintf(&b, "- %s: %s\n", t.Name, t.Description)
		if len(t.Required) > 0 {
			fmt.Fprintf(&b, "  required args: %s\n", strings.Join(t.Required, ", "))
		}
		if len(t.Optional) > 0 {
			fmt.Fprintf(&b, "  optional args: %s\n", strings.Join(t.Optional, ", "))
		}
	}
	return b.String()
}
