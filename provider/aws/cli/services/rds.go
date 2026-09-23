package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/rds"
	"context"
)

type dbListCmd struct {
	globals.AWSCLIFlag
}

type dbDefinitionCmd struct {
	globals.AWSCLIFlag
	Identifier string `name:"name" arg:"required" help:"DB instance or cluster identifier"`
}

type RDSCommand struct {
	List         dbListCmd       `name:"ls" cmd:"" help:"Return list of RDS instances and clusters"`
	DBDefinition dbDefinitionCmd `name:"def" cmd:"" help:"Return RDS instance/cluster definition"`
}

func (cmd *dbListCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := rds.NewDBListCommandExecutor(*session)
	return icmd.Execute(ctx)
}

func (cmd *dbDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := rds.NewDBDefinitionCommandExecutor(*session, cmd.Identifier)
	return icmd.Execute(ctx)
}
