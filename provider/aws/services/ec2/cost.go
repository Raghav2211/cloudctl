package ec2

import (
	"cloudctl/cost"
	"cloudctl/evidence"
	ctltime "cloudctl/time"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
)

const ruleEC2IdleLowCPU = "ec2-idle-low-cpu"

// InvestigateIdleInstances scans every running EC2 instance's recent CPU
// utilization — the same account-wide fetch as `ctl aws ec2 stats`, not a
// new CloudWatch call — and flags any classified CPU_LOW as a likely-idle
// cost-waste candidate. Used by `ctl aws cost`'s idle-scan fallback when
// Cost Explorer access isn't available.
func InvestigateIdleInstances(ctx context.Context, cfg aws.Config) ([]cost.Finding, int, error) {
	fetcher := statisticsFetcher{
		client:           awsec2.NewFromConfig(cfg),
		cloudwatchClient: cloudwatch.NewFromConfig(cfg),
		tz:               ctltime.GetTZ("UTC"),
	}
	out, err := fetcher.Fetch(ctx)
	if err != nil {
		return nil, 0, err
	}

	var findings []cost.Finding
	for _, s := range out.stats {
		if f := instanceIdleFinding(s); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings, len(out.stats), nil
}

// instanceIdleFinding returns a Finding if s is classified as low-CPU
// (likely idle), or nil if it's healthy, unclassifiable, or its CloudWatch
// fetch failed.
func instanceIdleFinding(s instanceStatisticsOutput) *cost.Finding {
	if s.apiError != nil || s.Average == nil || s.CPUStatus != CPU_LOW {
		return nil
	}
	id := derefStr(s.instanceId)
	return &cost.Finding{
		Rule:           ruleEC2IdleLowCPU,
		ResourceID:     id,
		Description:    fmt.Sprintf("Average CPU utilization over the last 2 days is %.1f%% (classified Low).", *s.Average),
		Recommendation: "Consider stopping or downsizing this instance if it's not doing meaningful work.",
		Evidence: []evidence.Evidence{
			{Source: "cloudwatch:GetMetricStatistics", ResourceID: id, Field: "CPUPercentAvg48h", Value: *s.Average, Confidence: evidence.Fact},
		},
	}
}
