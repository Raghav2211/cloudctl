package security

import (
	"cloudctl/viewer"
	"fmt"
	"sort"
)

var severityRank = map[Severity]int{Critical: 0, High: 1, Medium: 2, Low: 3}

var findingsTableHeader = viewer.Row{"Severity", "Rule", "Description", "Remediation"}

// Viewer renders a Report: a clean "no findings" panel if every rule
// checked passed, otherwise a table sorted most-severe-first — never a
// silent list a reader has to re-sort themselves to prioritize.
func Viewer(report *Report, err error) viewer.Viewer {
	if err != nil {
		ev := viewer.NewErrorViewer()
		ev.SetErrorType(viewer.ERROR)
		ev.SetErrorMessage(err.Error())
		return ev
	}

	if len(report.Findings) == 0 {
		p := viewer.NewPanel().SetTitle(fmt.Sprintf("Security findings for %s", report.ResourceID))
		p.SetBody("no findings — every rule checked passed")
		return p
	}

	findings := make([]Finding, len(report.Findings))
	copy(findings, report.Findings)
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
	})

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Security findings for %s (%d)", report.ResourceID, len(findings)))
	tv.AddHeader(findingsTableHeader)
	for _, f := range findings {
		tv.AddRow(viewer.Row{string(f.Severity), f.Rule, f.Description, f.Remediation})
	}
	return tv
}
