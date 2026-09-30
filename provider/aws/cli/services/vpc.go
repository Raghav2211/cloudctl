package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/vpc"
	"context"
)

type vpcListCmd struct {
	globals.AWSCLIFlag
}

type vpcDefinitionCmd struct {
	globals.AWSCLIFlag
	VpcID string `name:"id" arg:"required" help:"VPC ID"`
}

type vpcSecurityCmd struct {
	globals.AWSCLIFlag
	VpcID string `name:"id" arg:"required" help:"VPC ID"`
}

type VPCCommand struct {
	List          vpcListCmd       `name:"ls" cmd:"" help:"Return list of VPCs"`
	VPCDefinition vpcDefinitionCmd `name:"def" cmd:"" help:"Return VPC topology (subnets, NAT/internet gateways)"`
	Security      vpcSecurityCmd   `name:"security" cmd:"" help:"Run deterministic security checks (e.g. default VPC in use)"`
}

func (cmd *vpcListCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := vpc.NewVPCListCommandExecutor(*session)
	return icmd.Execute(ctx)
}

func (cmd *vpcDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := vpc.NewVPCDefinitionCommandExecutor(*session, cmd.VpcID)
	return icmd.Execute(ctx)
}

func (cmd *vpcSecurityCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := vpc.NewVPCSecurityCommandExecutor(*session, cmd.VpcID)
	return icmd.Execute(ctx)
}
