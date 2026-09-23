package dynamodb

import (
	"cloudctl/executor"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func NewTableListCommandExecutor(cfg aws.Config) *executor.CommandExecutor[*tableListOutput] {
	return &executor.CommandExecutor[*tableListOutput]{
		Fetcher: &tableListFetcher{
			client: dynamodb.NewFromConfig(cfg),
		},
		Viewer: tableListViewer,
	}
}

func NewTableDefinitionCommandExecutor(cfg aws.Config, tableName string) *executor.CommandExecutor[*tableDefinition] {
	return &executor.CommandExecutor[*tableDefinition]{
		Fetcher: &tableDefinitionFetcher{
			client:    dynamodb.NewFromConfig(cfg),
			tableName: tableName,
		},
		Viewer: tableDefinitionViewer,
	}
}
