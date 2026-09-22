package ec2

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/snapshot"
	"cloudctl/time"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func NewinstanceListCommandExecutor(flag *globals.AWSCLIFlag, tzIdentifier string, filter InstanceListFilter) (*executor.CommandExecutor[*instanceListOutput], error) {
	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(*flag, true))
	if err != nil {
		return nil, err
	}
	return &executor.CommandExecutor[*instanceListOutput]{
		Fetcher: &instanceListFetcher{
			client: ec2.NewFromConfig(*cfg),
			tz:     time.GetTZ(tzIdentifier),
			filter: filter,
		},
		Viewer: instanceListViewer,
	}, nil
}

// NewInstanceListFromSnapshotCommandExecutor reads instances from the latest
// local snapshot (see `ctl discover aws`) instead of calling AWS live. Only
// the client-side hasPublicIp filter applies here — the state/type/az/vpc/
// subnet/launchAt filters are server-side AWS API filters with no snapshot
// equivalent in this MVP.
func NewInstanceListFromSnapshotCommandExecutor(tzIdentifier string, filter InstanceListFilter) (*executor.CommandExecutor[*instanceListOutput], error) {
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		return nil, err
	}
	return &executor.CommandExecutor[*instanceListOutput]{
		Fetcher: &instanceListFromSnapshotFetcher{
			store:  store,
			tz:     time.GetTZ(tzIdentifier),
			filter: filter,
		},
		Viewer: instanceListViewer,
	}, nil
}

func NewInstanceDescribeCommandExecutor(flag *globals.AWSCLIFlag, tzIdentifier string, instanceId string) (*executor.CommandExecutor[*instanceDefinition], error) {
	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(*flag, true))
	if err != nil {
		return nil, err
	}
	spaceTrimmedInstanceId := strings.TrimSpace(instanceId)
	return &executor.CommandExecutor[*instanceDefinition]{
		Fetcher: &instanceDefinitionFetcher{
			client: ec2.NewFromConfig(*cfg),
			id:     &spaceTrimmedInstanceId,
			tz:     time.GetTZ(tzIdentifier),
		},
		Viewer: instanceInfoViewer,
	}, nil
}

func NewEC2StatisticsDescribeCommandExecutor(flag *globals.AWSCLIFlag, tzIdentifier string) (*executor.CommandExecutor[*instanceStatisticsListOutput], error) {
	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(*flag, true))
	if err != nil {
		return nil, err
	}
	return &executor.CommandExecutor[*instanceStatisticsListOutput]{
		Fetcher: &statisticsFetcher{
			client:           ec2.NewFromConfig(*cfg),
			cloudwatchClient: cloudwatch.NewFromConfig(*cfg),
			tz:               time.GetTZ(tzIdentifier),
		},
		Viewer: ec2StatisticsViewer,
	}, nil
}

func NewSecurityGroupExplainCommandExecutor(flag *globals.AWSCLIFlag, sgId string) (*executor.CommandExecutor[*sgExplanation], error) {
	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(*flag, true))
	if err != nil {
		return nil, err
	}
	return &executor.CommandExecutor[*sgExplanation]{
		Fetcher: &sgExplainFetcher{
			client: ec2.NewFromConfig(*cfg),
			sgId:   strings.TrimSpace(sgId),
		},
		Viewer: sgExplainViewer,
	}, nil
}
