package eks

import (
	"cloudctl/executor"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

func NewClusterListCommandExecutor(cfg aws.Config) *executor.CommandExecutor[*clusterListOutput] {
	return &executor.CommandExecutor[*clusterListOutput]{
		Fetcher: &clusterListFetcher{
			client: eks.NewFromConfig(cfg),
		},
		Viewer: clusterListViewer,
	}
}

func NewClusterDefinitionCommandExecutor(cfg aws.Config, clusterName string) *executor.CommandExecutor[*clusterDefinition] {
	return &executor.CommandExecutor[*clusterDefinition]{
		Fetcher: &clusterDefinitionFetcher{
			client:      eks.NewFromConfig(cfg),
			clusterName: clusterName,
		},
		Viewer: clusterDefinitionViewer,
	}
}
