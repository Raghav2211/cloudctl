package viewer

import (
	"encoding/json"
	"fmt"
)

// Structurable is implemented by a Viewer that can also render as
// machine-readable structured data, for --output json/yaml. Not every
// Viewer needs to implement it — the executor falls back to the normal
// table/panel render if one doesn't.
type Structurable interface {
	Structured() any
}

// StructuredJSON renders v's Structured() output as compact JSON, or ""
// if v doesn't implement Structurable or fails to encode. Lets callers that
// need a plain-text snapshot of a view's data — e.g. the investigation
// agent's step history — reuse the same structured representation
// --output json/yaml already produces, instead of hand-writing a
// per-command summary.
func StructuredJSON(v Viewer) string {
	sv, ok := v.(Structurable)
	if !ok {
		return ""
	}
	b, err := json.Marshal(sv.Structured())
	if err != nil {
		return ""
	}
	return string(b)
}

type structuredTable struct {
	Title   string     `json:"title,omitempty" yaml:"title,omitempty"`
	Headers []string   `json:"headers,omitempty" yaml:"headers,omitempty"`
	Rows    [][]string `json:"rows" yaml:"rows"`
}

func (t *TableViewer) Structured() any {
	headers := make([]string, len(t.header))
	for i, h := range t.header {
		headers[i] = fmt.Sprintf("%v", h)
	}
	rows := make([][]string, len(t.rows))
	for i, row := range t.rows {
		r := make([]string, len(row))
		for j, cell := range row {
			r[j] = fmt.Sprintf("%v", cell)
		}
		rows[i] = r
	}
	return structuredTable{Title: t.title, Headers: headers, Rows: rows}
}

type structuredPanel struct {
	Title   string            `json:"title,omitempty" yaml:"title,omitempty"`
	Body    string            `json:"body,omitempty" yaml:"body,omitempty"`
	Entries map[string]string `json:"entries,omitempty" yaml:"entries,omitempty"`
}

func (p *Panel) Structured() any {
	var entries map[string]string
	if len(p.entries) > 0 {
		entries = make(map[string]string, len(p.entries))
		for _, e := range p.entries {
			entries[e.Label] = e.Value
		}
	}
	return structuredPanel{Title: p.title, Body: p.body, Entries: entries}
}

// Structured on CompoundViewer flattens to a list of each structurable
// child's own Structured() output, in render order — a child that doesn't
// implement Structurable (there are none today, but the check is cheap
// insurance) is silently skipped rather than breaking the whole encode.
func (v *CompoundViewer) Structured() any {
	parts := make([]any, 0, len(v.viewers))
	for _, sub := range v.viewers {
		if s, ok := sub.(Structurable); ok {
			parts = append(parts, s.Structured())
		}
	}
	return parts
}

type structuredTree struct {
	Title    string `json:"title,omitempty" yaml:"title,omitempty"`
	Root     string `json:"root" yaml:"root"`
	Children []any  `json:"children,omitempty" yaml:"children,omitempty"`
}

func (t *Tree) Structured() any {
	children := make([]any, 0, len(t.children))
	for _, c := range t.children {
		if nested, ok := c.(*Tree); ok {
			children = append(children, nested.Structured())
		} else {
			children = append(children, fmt.Sprintf("%v", c))
		}
	}
	return structuredTree{Title: t.title, Root: t.rootLabel, Children: children}
}

type structuredError struct {
	Error string `json:"error" yaml:"error"`
}

func (e *ErrorViewer) Structured() any {
	return structuredError{Error: e.message}
}
