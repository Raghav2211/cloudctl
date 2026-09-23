package vpc

import (
	"cloudctl/executor"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func NewVPCListCommandExecutor(cfg aws.Config) *executor.CommandExecutor[*vpcListOutput] {
	return &executor.CommandExecutor[*vpcListOutput]{
		Fetcher: &vpcListFetcher{
			client: ec2.NewFromConfig(cfg),
		},
		Viewer: vpcListViewer,
	}
}

func NewVPCDefinitionCommandExecutor(cfg aws.Config, vpcID string) *executor.CommandExecutor[*vpcDefinition] {
	return &executor.CommandExecutor[*vpcDefinition]{
		Fetcher: &vpcDefinitionFetcher{
			client: ec2.NewFromConfig(cfg),
			vpcID:  vpcID,
		},
		Viewer: vpcDefinitionViewer,
	}
}
