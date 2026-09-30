package cost

import (
	"strings"
	"testing"

	"cloudctl/viewer"
)

func TestViewer_CostExplorerMode(t *testing.T) {
	report := &Report{
		Mode: ModeCostExplorer,
		Summary: &CostSummary{
			PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", TotalAmount: 123.45, Unit: "USD",
			ByService: []ServiceCost{{Service: "Amazon EC2", Amount: 100.0, Unit: "USD"}, {Service: "Amazon S3", Amount: 23.45, Unit: "USD"}},
		},
	}
	v := Viewer(report, nil)
	json := viewer.StructuredJSON(v)
	if !strings.Contains(json, "Amazon EC2") {
		t.Errorf("expected the service breakdown in the rendered output, got %s", json)
	}
	v.View() // must not panic
}

func TestViewer_IdleScanMode_WithFindings(t *testing.T) {
	report := &Report{
		Mode:           ModeIdleScan,
		FallbackReason: "AccessDeniedException",
		Scanned:        5,
		Findings: []Finding{
			{Rule: "ec2-idle-low-cpu", ResourceID: "i-abc123", Description: "low CPU", Recommendation: "stop it"},
		},
	}
	v := Viewer(report, nil)
	json := viewer.StructuredJSON(v)
	if !strings.Contains(json, "i-abc123") {
		t.Errorf("expected the finding in the rendered output, got %s", json)
	}
	v.View() // must not panic
}

func TestViewer_IdleScanMode_NoFindings(t *testing.T) {
	report := &Report{Mode: ModeIdleScan, FallbackReason: "AccessDeniedException", Scanned: 5}
	v := Viewer(report, nil)
	v.View() // must not panic
	if v.IsFailure() {
		t.Error("expected a clean idle scan to not be a failure")
	}
}

func TestViewer_ErrorPropagates(t *testing.T) {
	v := Viewer(nil, errTest{})
	if !v.IsErrorView() {
		t.Error("expected an error to render as an error view")
	}
}

type errTest struct{}

func (errTest) Error() string { return "boom" }
