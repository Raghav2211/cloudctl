package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/rds"
	"context"
	"time"
)

type dbListCmd struct {
	globals.AWSCLIFlag
}

type dbDefinitionCmd struct {
	globals.AWSCLIFlag
	Identifier string `name:"name" arg:"required" help:"DB instance or cluster identifier"`
}

type dbStatisticsCmd struct {
	globals.AWSCLIFlag
	Identifier string `name:"name" arg:"required" help:"DB instance identifier"`
}

type dbEventsCmd struct {
	globals.AWSCLIFlag
	Identifier string        `name:"name" arg:"required" help:"DB instance identifier"`
	Since      time.Duration `name:"since" default:"24h" help:"How far back to search (RDS retains up to 14 days)"`
}

type dbSecurityCmd struct {
	globals.AWSCLIFlag
	Identifier string `name:"name" arg:"required" help:"DB instance or cluster identifier"`
}

type RDSCommand struct {
	List         dbListCmd       `name:"ls" cmd:"" help:"Return list of RDS instances and clusters"`
	DBDefinition dbDefinitionCmd `name:"def" cmd:"" help:"Return RDS instance/cluster definition"`
	Statistics   dbStatisticsCmd `name:"stats" cmd:"" help:"Return DB instance statistics (CPU, connections, storage, latency; last 24h)"`
	Events       dbEventsCmd     `name:"events" cmd:"" help:"Return RDS lifecycle events (backups, failovers, restarts, ...)"`
	Security     dbSecurityCmd   `name:"security" cmd:"" help:"Run deterministic security checks (e.g. public accessibility)"`
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

func (cmd *dbStatisticsCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := rds.NewDBStatisticsCommandExecutor(*session, cmd.Identifier)
	return icmd.Execute(ctx)
}

func (cmd *dbEventsCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := rds.NewDBEventListCommandExecutor(*session, cmd.Identifier, cmd.Since)
	return icmd.Execute(ctx)
}

func (cmd *dbSecurityCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := rds.NewDBSecurityCommandExecutor(*session, cmd.Identifier)
	return icmd.Execute(ctx)
}
