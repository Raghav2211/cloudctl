package s3

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
	"sort"
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
	return viewer.FuncViewer(data.Pretty)
}
