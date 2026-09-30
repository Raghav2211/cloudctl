package rds

import (
	"cloudctl/executor"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
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

func NewDBStatisticsCommandExecutor(cfg aws.Config, identifier string) *executor.CommandExecutor[*dbStatistics] {
	return &executor.CommandExecutor[*dbStatistics]{
		Fetcher: &dbStatisticsFetcher{
			client:     cloudwatch.NewFromConfig(cfg),
			identifier: identifier,
		},
		Viewer: dbStatisticsViewer,
	}
}

func NewDBEventListCommandExecutor(cfg aws.Config, identifier string, since time.Duration) *executor.CommandExecutor[*dbEventListOutput] {
	return &executor.CommandExecutor[*dbEventListOutput]{
		Fetcher: &dbEventListFetcher{
			client:     rds.NewFromConfig(cfg),
			identifier: identifier,
			since:      since,
		},
		Viewer: dbEventListViewer,
	}
}
