package ec2

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/time"
)

func NewinstanceListCommandExecutor(flag *globals.CLIFlag, instanceType []string) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)
	return &executor.CommandExecutor{
		Fetcher: &instanceListFetcher{
			client: client,
			tz:     time.GetTZ(flag.TZShortIdentifier),
			filter: instanceListFilter{instanceTypes: instanceType},
		},
		Viewer: instanceListViewer,
	}
}

func NewinstanceDescribeCommandExecutor(flag *globals.CLIFlag, instanceId string) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &instanceDefinitionFetcher{
			client: client,
			id:     &instanceId,
		},
		Viewer: instanceInfoViewer,
	}
}
