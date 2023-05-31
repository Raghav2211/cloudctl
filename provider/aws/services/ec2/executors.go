package ec2

import (
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services"

	"github.com/aws/aws-sdk-go/service/ec2"
)

func NewinstanceListCommandExecutor(flag *globals.CLIFlag) *services.CommandExecutor {
	client, err := newClient(flag.Profile, flag.Region, flag.Debug)
	if err != nil {
		panic(err)
	}
	return &services.CommandExecutor{
		Fetcher: &instanceListFetcher{
			client: client,
		},
		Viewer: instanceListViewer,
	}
}

func NewinstanceDescribeCommandExecutor(flag *globals.CLIFlag, instanceId string) *services.CommandExecutor {
	client, err := newClient(flag.Profile, flag.Region, flag.Debug)
	if err != nil {
		panic(err)
	}
	return &services.CommandExecutor{
		Fetcher: &instanceInfoFetcher{
			client: client,
			id:     &instanceId,
		},
		Viewer: instanceInfoViewer,
	}
}

func newClient(profile, region string, debug bool) (client *ec2.EC2, err error) {
	session, err := aws.NewSession(
		profile,
		region,
		debug,
	)
	if err != nil {
		return nil, err
	}
	client = ec2.New(session)
	return
}
