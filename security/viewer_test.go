package security

import (
	"cloudctl/viewer"
	"strings"
	"testing"
)

func TestViewer_NoFindings(t *testing.T) {
	v := Viewer(&Report{ResourceID: "i-abc123"}, nil)
	v.View() // must not panic
	if v.IsFailure() {
		t.Error("expected a clean report to not be a failure")
	}
}

func TestViewer_SortsMostSevereFirst(t *testing.T) {
	report := &Report{
		ResourceID: "i-abc123",
		Findings: []Finding{
			{Rule: "low-rule", Severity: Low},
			{Rule: "critical-rule", Severity: Critical},
			{Rule: "medium-rule", Severity: Medium},
			{Rule: "high-rule", Severity: High},
		},
	}
	view := Viewer(report, nil)
	json := viewer.StructuredJSON(view)
	if json == "" {
		t.Fatal("expected the findings table to implement Structurable")
	}
	order := []string{"critical-rule", "high-rule", "medium-rule", "low-rule"}
	last := -1
	for _, rule := range order {
		idx := strings.Index(json, rule)
		if idx == -1 {
			t.Fatalf("expected %q to appear in the rendered output, got %s", rule, json)
		}
		if idx <= last {
			t.Fatalf("expected severity order %v, but %q appeared out of order in %s", order, rule, json)
		}
		last = idx
	}
	view.View() // must not panic
}

func TestViewer_ErrorPropagates(t *testing.T) {
	v := Viewer(nil, errTest{})
	if !v.IsErrorView() {
		t.Error("expected an error Report to render as an error view")
	}
}

type errTest struct{}

func (errTest) Error() string { return "boom" }
