package s3

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
	"sort"
	"strings"
)

var (
	bucketListTableHeader = viewer.Row{
		"Name",
		"CreationDate",
	}
	bucketObjectsTableHeader = viewer.Row{
		"Key",
		"Size(Bytes)",
		"StorageClass",
		"LastModified",
	}
	bucketObjectsDownloadSummaryTableHeader = viewer.Row{
		"source",
		"destination",
		"size(bytes)",
		"timeElapsed",
		"error",
	}
)

func bucketListViewer(data *bucketListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.AddHeader(bucketListTableHeader)
	tViewer.SetTitle("Buckets")
	for _, bucket := range data.buckets {
		tViewer.AddRow(viewer.Row{
			*bucket.name,
			bucket.creationDate.String(),
		})
	}
	return tViewer
}

func bucketObjectsViewer(data *bucketObjectListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compoundViewer := viewer.NewCompoundViewer()
	if len(data.objects) > 0 {
		tViewer := viewer.NewTableViewer()
		tViewer.SetStyle(viewer.DefaultTableStyle())
		tViewer.AddHeader(bucketObjectsTableHeader)
		tViewer.SetTitle(*data.bucketName)

		// sort by LastModified DESC
		sort.Slice(data.objects, func(i, j int) bool {
			return data.objects[i].lastModified.After(*data.objects[j].lastModified)
		})

		for _, content := range data.objects {
			tViewer.AddRow(viewer.Row{
				*content.key,
				*content.sizeInBytes,
				*content.storageClass,
				*content.lastModified,
			})
		}
		compoundViewer.AddViewer(tViewer)
	}

	if data.notice != nil {
		compoundViewer.AddViewer(ctlaws.ErrorView(data.notice))
	}
	return compoundViewer
}

func bucketObjectsDownloadSummaryViewer(data *bucketOjectsDownloadSummary, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.AddHeader(bucketObjectsDownloadSummaryTableHeader)
	tViewer.SetTitle(fmt.Sprintf("[%s]: Download Summary", data.bucketName))
	for _, summary := range data.objectsDownloadSummary {
		if summary.err != nil {
			tViewer.AddRow(viewer.Row{
				summary.source,
				summary.destination,
				summary.sizeinBytes,
				summary.timeElapsed,
				summary.err.Err.Error(),
			})
		} else {
			tViewer.AddRow(viewer.Row{
				summary.source,
				summary.destination,
				summary.sizeinBytes,
				summary.timeElapsed,
				"N/A",
			})
		}

	}
	return tViewer

}

func bucketConfigurationViewer(data *bucketDefinition, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", *data.bucketName))
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

	dataPanel := viewer.NewPanel().SetTitle("Bucket Configuration")
	policyValue, policyOK := formatPolicy(data.policy)
	addBucketField(dataPanel, "Policy", policyValue, policyOK, data.policyAPIErr)
	versionValue, versionOK := formatVersioning(data.version)
	addBucketField(dataPanel, "Versioning", versionValue, versionOK, data.versionAPIErr)
	tagsValue, tagsOK := formatTags(data.tags)
	addBucketField(dataPanel, "Tags", tagsValue, tagsOK, data.tagsAPIError)
	encryptionValue, encryptionOK := formatEncryption(data.encryptionConfig)
	addBucketField(dataPanel, "Encryption", encryptionValue, encryptionOK, data.encryptionConfigAPIError)
	lifecycleValue, lifecycleOK := formatLifecycle(data.lifecycle)
	addBucketField(dataPanel, "Lifecycle", lifecycleValue, lifecycleOK, data.lifeCycleAPIError)
	compound.AddViewer(dataPanel)

	return compound
}

var bucketImpactTableHeader = viewer.Row{
	"Policy",
	"Effect",
	"Actions",
	"Principals",
}

func bucketImpactViewer(data *bucketImpact, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Risk Verdict for %s (Hypothesis — verify against the data below)", data.bucketName))
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

	if len(data.matches) == 0 {
		compound.AddViewer(viewer.NewPanel().SetTitle("IAM Cross-Reference").SetBody("no customer-managed IAM policies reference this bucket"))
		return compound
	}

	// Actions/Principals can legitimately be long (a broad admin policy's
	// action list, a policy attached to several principals) — unlike most
	// tables in this codebase, this one has a genuinely dominant wide
	// column, so an explicit MaxWidth is worth setting here (ADR-0019).
	style := viewer.DefaultTableStyle()
	style.MaxWidth = 160
	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(style)
	tViewer.SetTitle("IAM Cross-Reference (Inference — deterministic ARN match)")
	tViewer.AddHeader(bucketImpactTableHeader)
	for _, m := range data.matches {
		tViewer.AddRow(viewer.Row{m.policyName, m.effect, strings.Join(m.actions, ", "), strings.Join(m.principals, ", ")})
	}
	compound.AddViewer(tViewer)

	relPanel := viewer.NewPanel().SetTitle("Snapshot Store")
	if data.relationshipsError != "" {
		relPanel.SetBody("relationships not persisted: " + data.relationshipsError)
	} else {
		relPanel.SetBody(fmt.Sprintf("%d relationship edge(s) written to the snapshot store", data.relationshipsSaved))
	}
	compound.AddViewer(relPanel)

	return compound
}

// addBucketField adds one bucket-configuration field to a panel: its
// formatted value on success, the real fetch error on failure, or "none" if
// neither a value nor an error was ever recorded.
func addBucketField(panel *viewer.Panel, label string, formatted string, ok bool, apiErr error) {
	switch {
	case ok:
		panel.AddEntry(label, formatted)
	case apiErr != nil:
		panel.AddEntry(label, "error: "+apiErr.Error())
	default:
		panel.AddEntry(label, "none")
	}
}
