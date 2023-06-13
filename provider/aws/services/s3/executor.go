package s3

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"

	ctltime "cloudctl/time"
)

const (
	DATE_PASER = "2006-01-02 15:04:05"
)

func NewBucketListCommandExecutor(flag *globals.CLIFlag, filter *BucketListFilter) *executor.CommandExecutor {
	tz := ctltime.GetTZ(flag.TZShortIdentifier)

	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &bucketListFetcher{
			client: client,
			filter: filter,
			tz:     tz,
		},
		Viewer: bucketListViewer,
	}
}

func NewBucketObjectListCommandExecutor(flag *globals.CLIFlag, bucketName string, bucketPrefix *string, maxKeys int64) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &bucketObjectsFetcher{
			client:       client,
			bucketName:   bucketName,
			objectPrefix: bucketPrefix,
			maxKeys:      maxKeys,
			tz:           ctltime.GetTZ(flag.TZShortIdentifier),
		},
		Viewer: bucketObjectsViewer,
	}
}

func NewBucketViewCommandExecutor(flag *globals.CLIFlag, bucketName string) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &bucketConfigurationFetcher{
			client:     client,
			bucketName: bucketName,
		},
		Viewer: bucketConfigurationViewer,
	}
}

func NewBucketObjectDownloadCommandExecutor(flag *globals.CLIFlag, bucketName, key, path string, recursive bool) *executor.CommandExecutor {

	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &bucketObjectsDownloadFetcher{
			client:     client,
			bucketName: bucketName,
			key:        key,
			path:       path,
			recursive:  recursive,
		},
		Viewer: bucketObjectsDownloadSummaryViewer,
	}
}
