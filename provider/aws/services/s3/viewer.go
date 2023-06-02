package s3

import (
	"cloudctl/viewer"
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
	}
)

func bucketListViewer(o interface{}) viewer.Viewer {

	data := o.(*bucketListOutput)
	if data.err != nil {
		eView := viewer.ErrorViewer{}
		eView.SetErrorMessage(data.err.Err.Error())
		eView.SetErrorType(data.err.ErrorType)
		eView.SetErrorMeta(data.err.Meta)
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
	data := o.([]*objectDownloadSummary)
	tViewer := viewer.NewTableViewer()
	tViewer.AddHeader(bucketObjectsDownloadSummaryTableHeader)
	tViewer.SetTitle("Download Summary")
	for _, summary := range data {
		tViewer.AddRow(viewer.Row{
			summary.source,
			summary.destination,
			summary.sizeinBytes,
			summary.timeElapsed,
		})
	}
	return tViewer

}

func bucketConfigurationViewer(o interface{}) viewer.Viewer {
	o.(*bucketDefinition).Pretty()
	return nil
}
