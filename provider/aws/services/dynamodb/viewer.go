package dynamodb

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
)

var tableListTableHeader = viewer.Row{
	"Name",
}

func tableListViewer(data *tableListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.SetTitle("DynamoDB Tables")
	tViewer.AddHeader(tableListTableHeader)
	for _, t := range data.tables {
		tViewer.AddRow(viewer.Row{derefStr(t.name)})
	}
	return tViewer
}

func tableDefinitionViewer(data *tableDefinition, err error) viewer.Viewer {
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

	dataPanel := viewer.NewPanel().SetTitle("Table Configuration")
	dataPanel.AddEntry("Status", derefStr(data.status))
	dataPanel.AddEntry("Billing Mode", derefStr(data.billingMode))
	dataPanel.AddEntry("Encryption", derefStr(data.encryptionType))
	dataPanel.AddEntry("GSI Count", fmt.Sprintf("%d", data.gsiCount))
	dataPanel.AddEntry("LSI Count", fmt.Sprintf("%d", data.lsiCount))
	if data.itemCount != nil {
		dataPanel.AddEntry("Item Count", fmt.Sprintf("%d", *data.itemCount))
	} else {
		dataPanel.AddEntry("Item Count", "-")
	}
	stream := "disabled"
	if data.streamEnabled {
		stream = fmt.Sprintf("enabled (%s)", derefStr(data.streamViewType))
	}
	dataPanel.AddEntry("Stream", stream)
	addTableField(dataPanel, "Point-in-Time Recovery", derefStr(data.pitrStatus), data.pitrStatus != nil, data.pitrAPIErr)
	ttlValue := "-"
	if data.ttlStatus != nil {
		ttlValue = fmt.Sprintf("%s (attribute: %s)", derefStr(data.ttlStatus), derefStr(data.ttlAttribute))
	}
	addTableField(dataPanel, "Time to Live", ttlValue, data.ttlStatus != nil, data.ttlAPIErr)
	compound.AddViewer(dataPanel)

	return compound
}

// addTableField adds one table-configuration field to a panel: its
// formatted value on success, the real fetch error on failure, or "none" if
// neither a value nor an error was ever recorded — mirrors
// s3/viewer.go's addBucketField for the same reason (PITR/TTL are each an
// independent, separately-failable sub-fetch).
func addTableField(panel *viewer.Panel, label string, formatted string, ok bool, apiErr error) {
	switch {
	case ok:
		panel.AddEntry(label, formatted)
	case apiErr != nil:
		panel.AddEntry(label, "error: "+apiErr.Error())
	default:
		panel.AddEntry(label, "none")
	}
}
