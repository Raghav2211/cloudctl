package lambda

import (
	"cloudctl/executor"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func NewFunctionListCommandExecutor(cfg aws.Config) *executor.CommandExecutor[*functionListOutput] {
	return &executor.CommandExecutor[*functionListOutput]{
		Fetcher: &functionListFetcher{
			client: lambda.NewFromConfig(cfg),
		},
		Viewer: functionListViewer,
	}
}

func NewFunctionDefinitionCommandExecutor(cfg aws.Config, functionName string) *executor.CommandExecutor[*functionDefinition] {
	return &executor.CommandExecutor[*functionDefinition]{
		Fetcher: &functionDefinitionFetcher{
			client:       lambda.NewFromConfig(cfg),
			functionName: functionName,
		},
		Viewer: functionDefinitionViewer,
	}
}

func NewFunctionStatisticsCommandExecutor(cfg aws.Config, functionName string) *executor.CommandExecutor[*functionStatistics] {
	return &executor.CommandExecutor[*functionStatistics]{
		Fetcher: &functionStatisticsFetcher{
			client:       cloudwatch.NewFromConfig(cfg),
			functionName: functionName,
		},
		Viewer: functionStatisticsViewer,
	}
}

func NewFunctionLogsCommandExecutor(cfg aws.Config, functionName string, since time.Duration, limit int32, filterPattern string) *executor.CommandExecutor[*functionLogs] {
	return &executor.CommandExecutor[*functionLogs]{
		Fetcher: &functionLogsFetcher{
			client:        cloudwatchlogs.NewFromConfig(cfg),
			functionName:  functionName,
			since:         since,
			limit:         limit,
			filterPattern: filterPattern,
		},
		Viewer: functionLogsViewer,
	}
}
