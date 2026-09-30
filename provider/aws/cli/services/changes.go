package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/changes"
	"context"
	"time"
)

type ChangesAWSCmd struct {
	globals.AWSCLIFlag
	Resource      string        `name:"resource" help:"Filter to changes referencing this resource name/ID"`
	Event         string        `name:"event" help:"Filter to only this CloudTrail event name (e.g. RunInstances)"`
	ExcludeEvents []string      `name:"exclude-event" help:"Exclude this event name from results (repeatable, e.g. --exclude-event AssumeRole --exclude-event ConsoleLogin)"`
	Since         time.Duration `name:"since" default:"2h" help:"How far back to search (e.g. 30m, 2h, 24h)"`
	Limit         int32         `name:"limit" default:"50" help:"Maximum number of change events to return"`
}

// Run looks up recent CloudTrail management events — the roadmap's "what
// changed before the problem started" capability. Resolves credentials
// exactly like every other AWS command.
func (cmd *ChangesAWSCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := changes.NewChangeListCommandExecutor(*session, cmd.Resource, cmd.Event, cmd.ExcludeEvents, cmd.Since, cmd.Limit)
	return icmd.Execute(ctx)
}
