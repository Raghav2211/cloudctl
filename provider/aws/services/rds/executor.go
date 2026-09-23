package rds

import (
	"cloudctl/executor"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

func NewDBListCommandExecutor(cfg aws.Config) *executor.CommandExecutor[*dbListOutput] {
	return &executor.CommandExecutor[*dbListOutput]{
		Fetcher: &dbListFetcher{
			client: rds.NewFromConfig(cfg),
		},
		Viewer: dbListViewer,
	}
}

func NewDBDefinitionCommandExecutor(cfg aws.Config, identifier string) *executor.CommandExecutor[*dbDefinition] {
	return &executor.CommandExecutor[*dbDefinition]{
		Fetcher: &dbDefinitionFetcher{
			client:     rds.NewFromConfig(cfg),
			identifier: identifier,
		},
		Viewer: dbDefinitionViewer,
	}
}
