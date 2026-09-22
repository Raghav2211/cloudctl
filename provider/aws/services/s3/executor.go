package s3

import (
	"cloudctl/executor"
	"cloudctl/global"
	"cloudctl/provider/aws/cli/globals"

	ctltime "cloudctl/time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	DATE_PASER = "2006-01-02 15:04:05"
)

func NewBucketListCommandExecutor(flag global.CLIFlag, cfg aws.Config, timeout globals.RequestTimeout, filter *BucketListFilter) *executor.CommandExecutor[*bucketListOutput] {
	tz := ctltime.GetTZ(flag.TZShortIdentifier)

	return &executor.CommandExecutor[*bucketListOutput]{
		Fetcher: &bucketListFetcher{
			client:         s3.NewFromConfig(cfg),
			requestTimeout: timeout,
			filter:         filter,
			tz:             tz,
		},
		Viewer: bucketListViewer,
	}
}

func NewBucketObjectListCommandExecutor(cfg aws.Config, bucketName string, bucketPrefix *string, maxKeys int32) *executor.CommandExecutor[*bucketObjectListOutput] {
	return &executor.CommandExecutor[*bucketObjectListOutput]{
		Fetcher: &bucketObjectsFetcher{
			client:       s3.NewFromConfig(cfg),
			bucketName:   bucketName,
			objectPrefix: bucketPrefix,
			maxKeys:      maxKeys,
			tz:           ctltime.GetTZ(""),
		},
		Viewer: bucketObjectsViewer,
	}
}

func NewBucketViewCommandExecutor(cfg aws.Config, bucketName string) *executor.CommandExecutor[*bucketDefinition] {

	return &executor.CommandExecutor[*bucketDefinition]{
		Fetcher: &bucketConfigurationFetcher{
			client:     s3.NewFromConfig(cfg),
			bucketName: bucketName,
		},
		Viewer: bucketConfigurationViewer,
	}
}

func NewBucketObjectDownloadCommandExecutor(cfg aws.Config, bucketName, key, path string, recursive bool) *executor.CommandExecutor[*bucketOjectsDownloadSummary] {

	return &executor.CommandExecutor[*bucketOjectsDownloadSummary]{
		Fetcher: &bucketObjectsDownloadFetcher{
			client:     s3.NewFromConfig(cfg),
			downloader: manager.NewDownloader(s3.NewFromConfig(cfg)),
			bucketName: bucketName,
			key:        key,
			path:       path,
			recursive:  recursive,
		},
		Viewer: bucketObjectsDownloadSummaryViewer,
	}
}
