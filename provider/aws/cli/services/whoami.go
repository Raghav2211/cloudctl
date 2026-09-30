package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/identity"
	"context"
)

type WhoamiAWSCmd struct {
	globals.AWSCLIFlag
}

// Run resolves credentials exactly like every other AWS command, then
// reports who they resolve to via STS GetCallerIdentity — the fast way to
// confirm credentials/region/connectivity are all working before running
// anything else.
func (cmd *WhoamiAWSCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	credentialSource := ""
	if creds, credErr := session.Credentials.Retrieve(ctx); credErr == nil {
		credentialSource = creds.Source
	}

	icmd := identity.NewWhoamiCommandExecutor(*session, credentialSource)
	return icmd.Execute(ctx)
}
