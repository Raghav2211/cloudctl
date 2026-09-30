package services

import (
	"cloudctl/ai"
	"cloudctl/global"
	"cloudctl/nlquery"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/changes"
	"cloudctl/provider/aws/services/ec2"
	"context"
	"fmt"
	"time"
)

type AskCmd struct {
	globals.AWSCLIFlag
	Query string `arg:"" help:"A natural-language request, e.g. \"show running ec2 instances\""`
}

// Run classifies Query into one of a small, fixed set of supported intents
// (nlquery.Classify — validated against a hard whitelist, never trusted
// blindly) and dispatches the validated intent to the same executor an
// equivalent hand-typed command would use. The model never runs anything
// itself; it only ever selects from, and fills in flag values for, a fixed
// set of existing read-only operations — the roadmap's Phase 4 requirement
// that natural-language requests convert to "supported structured queries"
// and never directly invoke an arbitrary AWS API. Every interpreted action
// is printed before it runs, so the mapping is never a silent surprise.
func (cmd *AskCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	intent, err := nlquery.Classify(ctx, ai.NewClientFromEnv(), cmd.Query)
	if err != nil {
		return err
	}
	fmt.Printf("Interpreted as: %s\n", intent.Describe())

	switch intent.Command {
	case "find":
		return RunFind(ctx, intent.Flags["concept"])
	case "ec2":
		return runAskEC2LS(ctx, cmd.AWSCLIFlag, cli, intent)
	case "changes":
		return runAskChanges(ctx, cmd.AWSCLIFlag, cli, intent)
	default:
		// nlquery.Classify already rejects anything outside its whitelist,
		// so this is unreachable in practice — kept as a safety net against
		// the whitelist ever being extended without a matching case here.
		return fmt.Errorf("query not supported: %q is not a known command", intent.Command)
	}
}

func runAskEC2LS(ctx context.Context, awsFlag globals.AWSCLIFlag, cli *global.CLIFlag, intent *nlquery.Intent) error {
	var filters []ec2.InstanceListFilterOptFunc
	if state := intent.Flags["state"]; state != "" {
		filters = append(filters, ec2.WithInstanceStates([]string{state}))
	}
	if az := intent.Flags["az"]; az != "" {
		filters = append(filters, ec2.WithAvailabilityZone([]string{az}))
	}
	if vpc := intent.Flags["vpc"]; vpc != "" {
		filters = append(filters, ec2.WithVpcIds([]string{vpc}))
	}
	if intent.Flags["has_public_ip"] == "true" {
		filters = append(filters, ec2.WithHasPublicIp())
	}
	filter := ec2.NewInstanceFilter(filters...)

	icmd, err := ec2.NewinstanceListCommandExecutor(&awsFlag, cli.Debug, cli.TZShortIdentifier, *filter)
	if err != nil {
		return err
	}
	return icmd.Execute(ctx)
}

func runAskChanges(ctx context.Context, awsFlag globals.AWSCLIFlag, cli *global.CLIFlag, intent *nlquery.Intent) error {
	credentialConfig := aws.NewCredentialConfig(awsFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	since := 2 * time.Hour
	if s := intent.Flags["since"]; s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			since = d
		}
	}

	icmd := changes.NewChangeListCommandExecutor(*session, intent.Flags["resource"], intent.Flags["event"], nil, since, 50)
	return icmd.Execute(ctx)
}
