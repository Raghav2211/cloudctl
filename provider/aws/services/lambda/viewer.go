package lambda

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
	"strings"
)

var functionListTableHeader = viewer.Row{
	"Name",
	"Runtime",
	"State",
}

func functionListViewer(data *functionListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.SetTitle("Lambda Functions")
	tViewer.AddHeader(functionListTableHeader)
	for _, f := range data.functions {
		tViewer.AddRow(viewer.Row{derefStr(f.name), f.runtime, f.state})
	}
	return tViewer
}

func functionDefinitionViewer(data *functionDefinition, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", derefStr(data.name)))
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

	recommendationsPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Recommendations for %s (Recommendation — a human must apply these)", derefStr(data.name)))
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

	h := functionHealth(data)
	healthPanel := viewer.NewPanel().SetTitle("Health")
	healthBody := string(h.Status)
	if len(h.Reasons) > 0 {
		healthBody += ": " + strings.Join(h.Reasons, "; ")
	}
	healthPanel.SetBody(healthBody)
	compound.AddViewer(healthPanel)

	dataPanel := viewer.NewPanel().SetTitle("Function Configuration")
	dataPanel.AddEntry("Runtime", data.runtime)
	dataPanel.AddEntry("State", data.state)
	dataPanel.AddEntry("Package Type", data.packageType)
	dataPanel.AddEntry("Handler", derefStr(data.handler))
	dataPanel.AddEntry("Role", derefStr(data.role))
	if data.memorySizeMB != nil {
		dataPanel.AddEntry("Memory (MB)", fmt.Sprintf("%d", *data.memorySizeMB))
	}
	if data.timeoutSec != nil {
		dataPanel.AddEntry("Timeout (s)", fmt.Sprintf("%d", *data.timeoutSec))
	}
	dataPanel.AddEntry("Code Size (bytes)", fmt.Sprintf("%d", data.codeSizeBytes))
	dataPanel.AddEntry("Architectures", strings.Join(data.architectures, ", "))
	dataPanel.AddEntry("Last Modified", derefStr(data.lastModified))
	compound.AddViewer(dataPanel)

	return compound
}
