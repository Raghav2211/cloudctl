package rds

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
)

var dbListTableHeader = viewer.Row{
	"Identifier",
	"Kind",
	"Engine",
	"Status",
}

func dbListViewer(data *dbListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.SetTitle("RDS Instances & Clusters")
	tViewer.AddHeader(dbListTableHeader)
	for _, item := range data.items {
		tViewer.AddRow(viewer.Row{derefStr(item.identifier), item.kind, derefStr(item.engine), derefStr(item.status)})
	}
	return tViewer
}

func dbDefinitionViewer(data *dbDefinition, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", derefStr(data.identifier)))
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

	recommendationsPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Recommendations for %s (Recommendation — a human must apply these)", derefStr(data.identifier)))
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

	dataPanel := viewer.NewPanel().SetTitle("Database Configuration")
	dataPanel.AddEntry("Kind", data.kind)
	dataPanel.AddEntry("Engine", derefStr(data.engine))
	dataPanel.AddEntry("Engine Version", derefStr(data.engineVersion))
	dataPanel.AddEntry("Status", derefStr(data.status))
	if data.instanceClass != nil {
		dataPanel.AddEntry("Instance Class", derefStr(data.instanceClass))
	}
	if data.allocatedStorage != nil {
		dataPanel.AddEntry("Allocated Storage (GiB)", fmt.Sprintf("%d", *data.allocatedStorage))
	}
	dataPanel.AddEntry("Multi-AZ", fmt.Sprintf("%t", derefBool(data.multiAZ)))
	dataPanel.AddEntry("Storage Encrypted", fmt.Sprintf("%t", derefBool(data.storageEncrypted)))
	dataPanel.AddEntry("Publicly Accessible", fmt.Sprintf("%t", derefBool(data.publiclyAccessible)))
	if data.backupRetentionDays != nil {
		dataPanel.AddEntry("Backup Retention (days)", fmt.Sprintf("%d", *data.backupRetentionDays))
	}
	dataPanel.AddEntry("Endpoint", derefStr(data.endpoint))
	compound.AddViewer(dataPanel)

	return compound
}
