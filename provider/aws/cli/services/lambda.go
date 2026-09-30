package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/lambda"
	"context"
	"time"
)

type functionListCmd struct {
	globals.AWSCLIFlag
}

type functionDefinitionCmd struct {
	globals.AWSCLIFlag
	FunctionName string `name:"name" arg:"required" help:"Function name"`
}

type functionStatisticsCmd struct {
	globals.AWSCLIFlag
	FunctionName string `name:"name" arg:"required" help:"Function name"`
}

type functionLogsCmd struct {
	globals.AWSCLIFlag
	FunctionName string        `name:"name" arg:"required" help:"Function name"`
	Since        time.Duration `name:"since" default:"1h" help:"How far back to search (e.g. 30m, 2h)"`
	Limit        int32         `name:"limit" default:"50" help:"Maximum number of log events to return"`
	Filter       string        `name:"filter" help:"CloudWatch Logs filter pattern"`
}

type functionSecurityCmd struct {
	globals.AWSCLIFlag
	FunctionName string `name:"name" arg:"required" help:"Function name"`
}

type LambdaCommand struct {
	List               functionListCmd       `name:"ls" cmd:"" help:"Return list of Lambda functions"`
	FunctionDefinition functionDefinitionCmd `name:"def" cmd:"" help:"Return function definition"`
	Statistics         functionStatisticsCmd `name:"stats" cmd:"" help:"Return function invocation/error/throttle/duration statistics (last 24h)"`
	Logs               functionLogsCmd       `name:"logs" cmd:"" help:"Return recent log events for a function"`
	Security           functionSecurityCmd   `name:"security" cmd:"" help:"Run deterministic security checks (e.g. deprecated runtime)"`
}

func (cmd *functionListCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := lambda.NewFunctionListCommandExecutor(*session)
	return icmd.Execute(ctx)
}

func (cmd *functionDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := lambda.NewFunctionDefinitionCommandExecutor(*session, cmd.FunctionName)
	return icmd.Execute(ctx)
}

func (cmd *functionStatisticsCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := lambda.NewFunctionStatisticsCommandExecutor(*session, cmd.FunctionName)
	return icmd.Execute(ctx)
}

func (cmd *functionLogsCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := lambda.NewFunctionLogsCommandExecutor(*session, cmd.FunctionName, cmd.Since, cmd.Limit, cmd.Filter)
	return icmd.Execute(ctx)
}

func (cmd *functionSecurityCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := lambda.NewFunctionSecurityCommandExecutor(*session, cmd.FunctionName)
	return icmd.Execute(ctx)
}
