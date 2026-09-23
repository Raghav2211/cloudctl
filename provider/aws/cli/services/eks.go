package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/eks"
	"context"
)

type clusterListCmd struct {
	globals.AWSCLIFlag
}

type clusterDefinitionCmd struct {
	globals.AWSCLIFlag
	ClusterName string `name:"name" arg:"required" help:"Cluster name"`
}

type EKSCommand struct {
	List              clusterListCmd       `name:"ls" cmd:"" help:"Return list of EKS clusters"`
	ClusterDefinition clusterDefinitionCmd `name:"def" cmd:"" help:"Return EKS cluster definition"`
}

func (cmd *clusterListCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := eks.NewClusterListCommandExecutor(*session)
	return icmd.Execute(ctx)
}

func (cmd *clusterDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := eks.NewClusterDefinitionCommandExecutor(*session, cmd.ClusterName)
	return icmd.Execute(ctx)
}
