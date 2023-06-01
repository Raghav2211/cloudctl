package ec2

import (
	"cloudctl/executor"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
)

func NewinstanceListCommandExecutor(flag *globals.CLIFlag) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)
	return &executor.CommandExecutor{
		Fetcher: &instanceListFetcher{
			client: client,
		},
		Viewer: instanceListViewer,
	}
}

func NewinstanceDescribeCommandExecutor(flag *globals.CLIFlag, instanceId string) *executor.CommandExecutor {
	client := aws.NewClient(flag.Profile, flag.Region, flag.Debug)

	return &executor.CommandExecutor{
		Fetcher: &instanceInfoFetcher{
			client: client,
			id:     &instanceId,
		},
		Viewer: instanceInfoViewer,
	}
}
