package changes

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"strings"
	"time"
)

var changeListTableHeader = viewer.Row{
	"Time",
	"Event",
	"Source",
	"Actor",
	"Resources",
	"Description",
}

func changeListViewer(data *changeListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle("Change Timeline")
	tv.AddHeader(changeListTableHeader)
	for _, c := range data.changes {
		ts := "-"
		if c.eventTime != nil {
			ts = c.eventTime.UTC().Format(time.RFC3339)
		}
		tv.AddRow(viewer.Row{ts, derefStr(c.eventName), derefStr(c.eventSource), derefStr(c.username), strings.Join(c.resources, ", "), c.description})
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle("AI Summary for Change Timeline (Hypothesis — verify against the events below)")
	if data.aiSummary != "" {
		summaryPanel.SetBody(data.aiSummary)
	} else {
		reason := data.aiSummaryUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		summaryPanel.SetBody("summary unavailable: " + reason)
	}
	compound.AddViewer(summaryPanel)

	recommendationsPanel := viewer.NewPanel().SetTitle("AI Recommendations for Change Timeline (Recommendation — a human must apply these)")
	if data.aiRecommendations != "" {
		recommendationsPanel.SetBody(data.aiRecommendations)
	} else {
		reason := data.aiRecommendationsUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		recommendationsPanel.SetBody("recommendations unavailable: " + reason)
	}
	compound.AddViewer(recommendationsPanel)

	compound.AddViewer(tv)
	return compound
}

func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}
