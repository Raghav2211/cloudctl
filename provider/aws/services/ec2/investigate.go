package ec2

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	ctltime "cloudctl/time"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
)

// InvestigateInstanceList runs the same fetch as `ctl aws ec2 ls`, for use
// by the investigation agent (`ctl investigate`). Bypasses
// NewinstanceListCommandExecutor (which needs a CLI AWSCLIFlag the agent
// doesn't have) and builds the same fetcher directly, unfiltered, in UTC.
func InvestigateInstanceList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := &executor.CommandExecutor[*instanceListOutput]{
		Fetcher: &instanceListFetcher{
			client: awsec2.NewFromConfig(cfg),
			tz:     ctltime.GetTZ("UTC"),
			filter: *NewInstanceFilter(),
		},
		Viewer: instanceListViewer,
	}
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateInstanceDef runs the same fetch as `ctl aws ec2 def`.
func InvestigateInstanceDef(ctx context.Context, cfg aws.Config, instanceID string) ([]evidence.Evidence, string, error) {
	exec := &executor.CommandExecutor[*instanceDefinition]{
		Fetcher: &instanceDefinitionFetcher{
			client: awsec2.NewFromConfig(cfg),
			id:     &instanceID,
			tz:     ctltime.GetTZ("UTC"),
		},
		Viewer: instanceInfoViewer,
	}
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return instanceDefinitionEvidence(def), viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}
