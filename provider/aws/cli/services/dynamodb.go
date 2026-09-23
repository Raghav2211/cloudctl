package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/dynamodb"
	"context"
)

type tableListCmd struct {
	globals.AWSCLIFlag
}

type tableDefinitionCmd struct {
	globals.AWSCLIFlag
	TableName string `name:"name" arg:"required" help:"Table name"`
}

type DynamoDBCommand struct {
	List            tableListCmd       `name:"ls" cmd:"" help:"Return list of DynamoDB tables"`
	TableDefinition tableDefinitionCmd `name:"def" cmd:"" help:"Return table definition"`
}

func (cmd *tableListCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := dynamodb.NewTableListCommandExecutor(*session)
	return icmd.Execute(ctx)
}

func (cmd *tableDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := dynamodb.NewTableDefinitionCommandExecutor(*session, cmd.TableName)
	return icmd.Execute(ctx)
}
