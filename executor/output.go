package executor

import (
	"cloudctl/viewer"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// OutputFormat is the process-wide rendering mode: the normal lipgloss
// table/panel render, or machine-readable JSON/YAML for automation (a
// stated product principle — see docs/roadmap-gap-analysis.md Phase 0).
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
)

// currentFormat is set once at startup (main.go, right after flag parsing,
// before any command runs) and read by every CommandExecutor — a legitimate
// process-wide global, not per-command state, so every command picks it up
// without threading a parameter through every Run() call site.
var currentFormat = FormatTable

// SetOutputFormat sets the process-wide output format. Call once, before
// any command runs.
func SetOutputFormat(f OutputFormat) {
	currentFormat = f
}

// renderStructured renders view as JSON/YAML to stdout instead of the
// normal table/panel render, if the requested format isn't FormatTable and
// view supports it (viewer.Structurable). Returns false — meaning nothing
// was rendered, so the caller must fall back to view.View() — if the format
// is FormatTable, or if this particular Viewer doesn't implement
// Structurable.
func renderStructured(view viewer.Viewer) bool {
	if currentFormat == FormatTable {
		return false
	}
	sv, ok := view.(viewer.Structurable)
	if !ok {
		return false
	}
	data := sv.Structured()
	switch currentFormat {
	case FormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			fmt.Fprintln(os.Stderr, "encoding JSON output:", err)
			return false
		}
	case FormatYAML:
		out, err := yaml.Marshal(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "encoding YAML output:", err)
			return false
		}
		fmt.Print(string(out))
	default:
		return false
	}
	return true
}
