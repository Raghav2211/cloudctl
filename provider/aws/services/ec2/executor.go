package ec2

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/time"
	"strings"
)

func NewinstanceListCommandExecutor(flag *globals.CLIFlag, filter InstanceListFilter) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)
	return &executor.CommandExecutor{
		Fetcher: &instanceListFetcher{
			client: client,
			tz:     time.GetTZ(flag.TZShortIdentifier),
			filter: filter,
		},
		Viewer: instanceListViewer,
	}
}

func NewInstanceDescribeCommandExecutor(flag *globals.CLIFlag, instanceId string) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)
	spaceTrimmedInstanceId := strings.TrimSpace(instanceId)
	return &executor.CommandExecutor{
		Fetcher: &instanceDefinitionFetcher{
			client: client,
			id:     &spaceTrimmedInstanceId,
			tz:     time.GetTZ(flag.TZShortIdentifier),
		},
		Viewer: instanceInfoViewer,
	}
}

func NewEC2StatisticsDescribeCommandExecutor(flag *globals.CLIFlag) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)
	return &executor.CommandExecutor{
		Fetcher: &statisticsFetcher{
			client: client,
			tz:     time.GetTZ(flag.TZShortIdentifier),
		},
		Viewer: ec2StatisticsViewer,
	}
}
