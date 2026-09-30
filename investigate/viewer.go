package investigate

import (
	"cloudctl/viewer"
	"fmt"
)

var stepsTableHeader = viewer.Row{"Step", "Tool", "Args", "Result"}

// Viewer renders an Investigation: the model's final conclusion and the
// deterministic evidence-grounded AI summary/recommendations first (most
// useful at a glance), then the full step-by-step timeline underneath for
// anyone who wants to audit exactly what the agent did and why — never a
// silent black box.
func Viewer(inv *Investigation, err error) viewer.Viewer {
	if err != nil {
		return errorPanel(err)
	}

	compound := viewer.NewCompoundViewer()

	conclusionPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("Conclusion for %q (%s)", inv.Question, inv.StoppedReason))
	switch {
	case inv.Conclusion != "":
		conclusionPanel.SetBody(inv.Conclusion)
	case inv.StoppedReason == "ai_unavailable":
		conclusionPanel.SetBody("the AI backend was unreachable — no investigation could run. See the raw commands below (`ctl aws <service> ...`) to check manually.")
	default:
		conclusionPanel.SetBody("the agent did not reach an explicit conclusion — review the steps and evidence below.")
	}
	compound.AddViewer(conclusionPanel)

	summaryPanel := viewer.NewPanel().SetTitle("AI Summary (Hypothesis — verify against the evidence below)")
	if inv.Summary != "" {
		summaryPanel.SetBody(inv.Summary)
	} else {
		reason := inv.SummaryUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		summaryPanel.SetBody("summary unavailable: " + reason)
	}
	compound.AddViewer(summaryPanel)

	recommendationsPanel := viewer.NewPanel().SetTitle("AI Recommendations (Recommendation — a human must apply these)")
	if inv.Recommendations != "" {
		recommendationsPanel.SetBody(inv.Recommendations)
	} else {
		reason := inv.RecommendationsUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		recommendationsPanel.SetBody("recommendations unavailable: " + reason)
	}
	compound.AddViewer(recommendationsPanel)

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Investigation steps (%d evidence facts gathered)", len(inv.Evidence)))
	tv.AddHeader(stepsTableHeader)
	for i, s := range inv.Steps {
		result := s.Summary
		if s.Err != "" {
			result = "error: " + s.Err
		}
		tv.AddRow(viewer.Row{i + 1, s.Tool, fmt.Sprintf("%v", s.Args), result})
	}
	compound.AddViewer(tv)

	return compound
}

// errorPanel mirrors ctlaws.ErrorView without importing the aws provider
// package — investigate is provider-agnostic and must not depend on it.
func errorPanel(err error) viewer.Viewer {
	ev := viewer.NewErrorViewer()
	ev.SetErrorType(viewer.ERROR)
	ev.SetErrorMessage(err.Error())
	return ev
}
