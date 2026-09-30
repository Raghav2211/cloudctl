package cost

import (
	"cloudctl/viewer"
	"fmt"
)

var costTableHeader = viewer.Row{"Service", "Amount", "Unit"}
var resourceCostTableHeader = viewer.Row{"Service", "Resource", "Amount", "Unit"}
var findingsTableHeader = viewer.Row{"Rule", "Resource", "Description", "Recommendation"}

// Viewer renders a Report according to its Mode — a real cost-by-service
// table, or an idle-resource findings table with a clear notice explaining
// why the fallback ran, never a silent substitution.
func Viewer(report *Report, err error) viewer.Viewer {
	if err != nil {
		ev := viewer.NewErrorViewer()
		ev.SetErrorType(viewer.ERROR)
		ev.SetErrorMessage(err.Error())
		return ev
	}

	switch report.Mode {
	case ModeCostExplorer:
		return costSummaryViewer(report.Summary)
	case ModeIdleScan:
		return idleScanViewer(report)
	default:
		p := viewer.NewPanel().SetTitle("Cost Analysis")
		p.SetBody("no cost data available")
		return p
	}
}

func costSummaryViewer(summary *CostSummary) viewer.Viewer {
	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Cost by service, %s to %s (total: %.2f %s)", summary.PeriodStart, summary.PeriodEnd, summary.TotalAmount, summary.Unit))
	tv.AddHeader(costTableHeader)
	for _, s := range summary.ByService {
		tv.AddRow(viewer.Row{s.Service, fmt.Sprintf("%.2f", s.Amount), s.Unit})
	}
	if len(summary.ByResource) == 0 {
		return tv
	}

	rv := viewer.NewTableViewer()
	rv.SetStyle(viewer.DefaultTableStyle())
	rv.SetTitle("Cost by resource")
	rv.AddHeader(resourceCostTableHeader)
	for _, r := range summary.ByResource {
		rv.AddRow(viewer.Row{r.Service, r.ResourceID, fmt.Sprintf("%.2f", r.Amount), r.Unit})
	}

	compound := viewer.NewCompoundViewer()
	compound.AddViewer(tv)
	compound.AddViewer(rv)
	return compound
}

func idleScanViewer(report *Report) viewer.Viewer {
	compound := viewer.NewCompoundViewer()

	notice := viewer.NewPanel().SetTitle("Cost Explorer unavailable — showing idle-resource findings instead")
	notice.SetBody(report.FallbackReason)
	compound.AddViewer(notice)

	if len(report.Findings) == 0 {
		p := viewer.NewPanel().SetTitle(fmt.Sprintf("Idle-resource scan (%d resources checked)", report.Scanned))
		p.SetBody("no idle resources found")
		compound.AddViewer(p)
		return compound
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Idle-resource findings (%d of %d resources checked)", len(report.Findings), report.Scanned))
	tv.AddHeader(findingsTableHeader)
	for _, f := range report.Findings {
		tv.AddRow(viewer.Row{f.Rule, f.ResourceID, f.Description, f.Recommendation})
	}
	compound.AddViewer(tv)
	return compound
}
