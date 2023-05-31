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
)

func bucketListViewer(o interface{}) viewer.Viewer {
	tViewer := viewer.NewTableViewer()
	tViewer.AddHeader(bucketListTableHeader)
	tViewer.SetTitle("Buckets")
	data := o.(*bucketListOutput)
	for _, bucket := range data.buckets {
		tViewer.AddRow(viewer.Row{*bucket.name, bucket.creationDate.String()})
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
		tViewer.AddRow(viewer.Row{*content.key, *content.sizeInBytes, *content.storageClass, *content.lastModified})
	}

	return tViewer

}

func bucketConfigurationViewer(o interface{}) viewer.Viewer {
	o.(*bucketInfo).Pretty()
	return nil
}
