package s3

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"

	ctltime "cloudctl/time"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	DATE_PASER = "2006-01-02 15:04:05"
)

func NewBucketListCommandExecutor(flag *globals.CLIFlag, from string, to string) *executor.CommandExecutor {
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

	clients, err := newClients(flag.Profile, flag.Region, flag.Debug, newClient)
	if err != nil {
		// TODO handle error
		panic(err)
	}
	return &executor.CommandExecutor{
		Fetcher: &bucketListFetcher{
			client: clients[0].(*s3.S3),
			filter: &bucketListFilter{
				creationDateFrom: dFrom,
				creationDateTo:   dTo,
			},
			tz: tz,
		},
		Viewer: bucketListViewer,
	}
}

func NewBucketObjectListCommandExecutor(flag *globals.CLIFlag, bucketName, bucketPrefix string) *executor.CommandExecutor {
	clients, err := newClients(flag.Profile, flag.Region, flag.Debug, newClient)
	if err != nil {
		// TODO handle error
		panic(err)
	}
	return &executor.CommandExecutor{
		Fetcher: &bucketObjectsFetcher{
			client:       clients[0].(*s3.S3),
			bucketName:   bucketName,
			objectPrefix: bucketPrefix,
			tz:           ctltime.GetTZ(flag.TZShortIdentifier),
		},
		Viewer: bucketObjectsViewer,
	}
}

func NewBucketViewCommandExecutor(flag *globals.CLIFlag, bucketName string) *executor.CommandExecutor {
	clients, err := newClients(flag.Profile, flag.Region, flag.Debug, newClient)
	if err != nil {
		// TODO handle error
		panic(err)
	}
	return &executor.CommandExecutor{
		Fetcher: &bucketConfigurationFetcher{
			client:     clients[0].(*s3.S3),
			bucketName: bucketName,
		},
		Viewer: bucketConfigurationViewer,
	}
}

func NewBucketObjectDownloadCommandExecutor(flag *globals.CLIFlag, bucketName, key, path string, recursive bool) *executor.CommandExecutor {

	clients, err := newClients(flag.Profile, flag.Region, flag.Debug, newClient, newS3Downloader)
	if err != nil {
		// TODO : handle error
		panic("error occur during create client")
	}

	return &executor.CommandExecutor{
		Fetcher: &bucketObjectsDownloadFetcher{
			client:     clients[0].(*s3.S3),
			downloader: clients[1].(*s3manager.Downloader),
			bucketName: bucketName,
			key:        key,
			path:       path,
			recursive:  recursive,
		},
		Viewer: bucketObjectsDownloadSummaryViewer,
	}
}

func newClients(profile, region string, debug bool, funcs ...func(session *session.Session) interface{}) ([]interface{}, error) {
	session, err := aws.NewSession(
		profile,
		region,
		debug,
	)
	if err != nil {
		return nil, err
	}
	var clients []interface{}
	for _, f := range funcs {
		clients = append(clients, f(session))
	}
	return clients, nil
}

func newClient(session *session.Session) interface{} {
	client := s3.New(session)
	return client
}

func newS3Downloader(session *session.Session) interface{} {
	client := s3manager.NewDownloader(session)
	return client
}
