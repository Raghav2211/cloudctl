package s3

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"

	ctltime "cloudctl/time"
	"fmt"
	"time"
)

const (
	DATE_PASER = "2006-01-02 15:04:05"
)

func NewBucketListCommandExecutor(flag *globals.CLIFlag, from, to, bucketNameString string) *executor.CommandExecutor {
	tz := ctltime.GetTZ(flag.TZShortIdentifier)
	dFrom := new(time.Time)
	dTo := new(time.Time)
	if len(from) != 0 {
		time, err := time.Parse(DATE_PASER, from)
		// TODO : handle error
		if err != nil {
			panic(err)
		}
		dFrom = tz.AdaptTimezone(&time)
	}
	if len(to) != 0 {
		time, err := time.Parse(DATE_PASER, to)
		// TODO : handle error
		if err != nil {
			panic(err)
		}
		dTo = tz.AdaptTimezone(&time)
	}
	if (!dFrom.IsZero() && !dTo.IsZero()) && (dFrom.After(*dTo)) {
		// TODO : handle error
		panic(fmt.Sprintf("from can't be after to if both provide | from=%s , to=%s", from, to))
	}

	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &bucketListFetcher{
			client: client,
			filter: &bucketListFilter{
				creationDateFrom: dFrom,
				creationDateTo:   dTo,
				bucketNameString: &bucketNameString,
				tz:               tz,
			},
			tz: tz,
		},
		Viewer: bucketListViewer,
	}
}

func NewBucketObjectListCommandExecutor(flag *globals.CLIFlag, bucketName, bucketPrefix string) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &bucketObjectsFetcher{
			client:       client,
			bucketName:   bucketName,
			objectPrefix: bucketPrefix,
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
