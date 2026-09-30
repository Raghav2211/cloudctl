package rds

import (
	"cloudctl/cost"
	"cloudctl/evidence"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"golang.org/x/sync/errgroup"
)

// maxConcurrentIdleChecks bounds how many per-instance stats fetches run at
// once during an idle scan — see ADR-0018.
const maxConcurrentIdleChecks = 5

const ruleDBIdleLowUtilization = "rds-idle-low-utilization"
const dbIdleCPUPercentThreshold = 5.0
const dbIdleConnectionsThreshold = 1.0

// InvestigateIdleInstances lists every RDS instance/cluster and checks each
// one's CloudWatch stats (the same fetch as `ctl aws rds stats`) for
// sustained low CPU and near-zero connections, flagging any as a likely-
// idle cost-waste candidate. Used by `ctl aws cost`'s idle-scan fallback
// when Cost Explorer access isn't available. Per-instance stats fetches run
// with bounded concurrency (ADR-0018); a single instance's fetch failing
// doesn't fail the whole scan.
func InvestigateIdleInstances(ctx context.Context, cfg aws.Config) ([]cost.Finding, int, error) {
	listOut, err := NewDBListCommandExecutor(cfg).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, 0, err
	}

	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrentIdleChecks)
	findingsPerItem := make([][]cost.Finding, len(listOut.items))

	for i, item := range listOut.items {
		i, item := i, item
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			if item == nil || item.identifier == nil {
				return nil
			}
			stats, statsErr := NewDBStatisticsCommandExecutor(cfg, *item.identifier).Fetcher.Fetch(gCtx)
			if statsErr != nil || stats.apiError != nil {
				return nil // a single instance's fetch failing isn't fatal to the scan
			}
			findingsPerItem[i] = dbIdleFindings(stats)
			return nil
		})
	}
	_ = g.Wait()

	var findings []cost.Finding
	for _, f := range findingsPerItem {
		findings = append(findings, f...)
	}
	return findings, len(listOut.items), nil
}

func dbIdleFindings(stats *dbStatistics) []cost.Finding {
	if stats.cpuPercent == nil || stats.connectionsAvg == nil {
		return nil
	}
	if *stats.cpuPercent >= dbIdleCPUPercentThreshold || *stats.connectionsAvg >= dbIdleConnectionsThreshold {
		return nil
	}
	id := stats.identifier
	return []cost.Finding{{
		Rule:           ruleDBIdleLowUtilization,
		ResourceID:     id,
		Description:    fmt.Sprintf("Average CPU is %.1f%% and average connections is %.1f over the last 24h — this instance looks idle.", *stats.cpuPercent, *stats.connectionsAvg),
		Recommendation: "Consider stopping (if supported) or downsizing this instance, or investigate why it has near-zero traffic.",
		Evidence: []evidence.Evidence{
			{Source: "cloudwatch:GetMetricStatistics", ResourceID: id, Field: "CPUPercent24h", Value: *stats.cpuPercent, Confidence: evidence.Fact},
			{Source: "cloudwatch:GetMetricStatistics", ResourceID: id, Field: "AvgConnections24h", Value: *stats.connectionsAvg, Confidence: evidence.Fact},
		},
	}}
}
