package s3

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/provider/aws/cli/globals"
	ctltime "cloudctl/time"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// defaultInvestigateRequestTimeout mirrors AWSCLIFlag's own defaults
// (connect-timeout/read-timeout) — NewBucketListCommandExecutor normally
// takes these from CLI flags, which the investigation agent has none of.
var defaultInvestigateRequestTimeout = globals.RequestTimeout{ConnectionTimeout: 60, ReadTimeout: 120}

// InvestigateBucketList runs the same fetch as `ctl aws s3 ls`, for use by
// the investigation agent (`ctl investigate`). Bypasses
// NewBucketListCommandExecutor (which needs CLI-flag-only inputs the agent
// doesn't have) and builds the same fetcher directly with an unfiltered,
// default-timeout configuration.
func InvestigateBucketList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := &executor.CommandExecutor[*bucketListOutput]{
		Fetcher: &bucketListFetcher{
			client:         s3.NewFromConfig(cfg),
			requestTimeout: defaultInvestigateRequestTimeout,
			filter:         NewBucketListFilter(),
			tz:             ctltime.GetTZ(""),
		},
		Viewer: bucketListViewer,
	}
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateBucketDef runs the same fetch as `ctl aws s3 def`.
func InvestigateBucketDef(ctx context.Context, cfg aws.Config, bucketName string) ([]evidence.Evidence, string, error) {
	exec := NewBucketViewCommandExecutor(cfg, bucketName)
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	name := bucketName
	if def.bucketName != nil {
		name = *def.bucketName
	}
	facts := bucketDefinitionEvidence(name, def.policy, def.version, def.tags, def.encryptionConfig, def.lifecycle)
	return facts, viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}

// InvestigateBucketImpact runs the same fetch as `ctl aws s3 impact`.
func InvestigateBucketImpact(ctx context.Context, cfg aws.Config, bucketName string) ([]evidence.Evidence, string, error) {
	exec := NewBucketImpactCommandExecutor(cfg, bucketName)
	impact, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return bucketImpactEvidence(impact), viewer.StructuredJSON(exec.Viewer(impact, nil)), nil
}
