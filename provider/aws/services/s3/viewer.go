package s3

import (
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

func bucketListViewer(o interface{}) viewer.Viewer {

	data := o.(*bucketListOutput)
	if data.err != nil {
		eView := viewer.ErrorViewer{}
		eView.SetErrorMessage(data.err.Err.Error())
		eView.SetErrorType(data.err.ErrorType)
		return &eView
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

func bucketObjectsViewer(o interface{}) viewer.Viewer {
	data := o.(*bucketObjectListOutput)

	if data.err != nil {
		// compoundViewer := viewer.NewCompoundViewer()
		errViewer := viewer.NewErrorViewer()
		errViewer.SetErrorMessage(data.err.Err.Error())
		errViewer.SetErrorType(data.err.ErrorType)
		// compoundViewer.AddErrorViewer(errView)
		return errViewer

	}

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

	return tViewer

}

func bucketObjectsDownloadSummaryViewer(o interface{}) viewer.Viewer {
	data := o.(*bucketOjectsDownloadSummary)
	if data.err != nil {
		errViewer := viewer.NewErrorViewer()
		errViewer.SetErrorMessage(data.err.Err.Error())
		errViewer.SetErrorType(data.err.ErrorType)
		return errViewer
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

func bucketConfigurationViewer(o interface{}) viewer.Viewer {
	o.(*bucketDefinition).Pretty()
	return nil
}
