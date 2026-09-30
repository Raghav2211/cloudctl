package lambda

import (
	"cloudctl/cost"
	"cloudctl/evidence"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"golang.org/x/sync/errgroup"
)

// maxConcurrentIdleChecks bounds how many per-function stats fetches run at
// once during an idle scan — see ADR-0018.
const maxConcurrentIdleChecks = 5

const ruleFunctionIdleNoInvocations = "lambda-idle-no-invocations"
const functionIdleInvocationThreshold = 1.0

// InvestigateIdleFunctions lists every Lambda function and checks each
// one's CloudWatch stats (the same fetch as `ctl aws lambda stats`) for
// near-zero invocations over the last 24h, flagging any as unused. Used by
// `ctl aws cost`'s idle-scan fallback when Cost Explorer access isn't
// available. Per-function stats fetches run with bounded concurrency
// (ADR-0018); a single function's fetch failing doesn't fail the whole
// scan.
func InvestigateIdleFunctions(ctx context.Context, cfg aws.Config) ([]cost.Finding, int, error) {
	listOut, err := NewFunctionListCommandExecutor(cfg).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, 0, err
	}

	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrentIdleChecks)
	findingsPerItem := make([][]cost.Finding, len(listOut.functions))

	for i, item := range listOut.functions {
		i, item := i, item
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			if item == nil || item.name == nil {
				return nil
			}
			stats, statsErr := NewFunctionStatisticsCommandExecutor(cfg, *item.name).Fetcher.Fetch(gCtx)
			if statsErr != nil || stats.apiError != nil {
				return nil // a single function's fetch failing isn't fatal to the scan
			}
			findingsPerItem[i] = functionIdleFindings(stats)
			return nil
		})
	}
	_ = g.Wait()

	var findings []cost.Finding
	for _, f := range findingsPerItem {
		findings = append(findings, f...)
	}
	return findings, len(listOut.functions), nil
}

func functionIdleFindings(stats *functionStatistics) []cost.Finding {
	if stats.invocations == nil || *stats.invocations >= functionIdleInvocationThreshold {
		return nil
	}
	id := stats.functionName
	return []cost.Finding{{
		Rule:           ruleFunctionIdleNoInvocations,
		ResourceID:     id,
		Description:    fmt.Sprintf("%.0f invocations over the last 24h — this function looks unused.", *stats.invocations),
		Recommendation: "If this function is no longer needed, remove it; if it's meant to be event-driven, verify its trigger is still configured correctly.",
		Evidence: []evidence.Evidence{
			{Source: "cloudwatch:GetMetricStatistics", ResourceID: id, Field: "Invocations24h", Value: *stats.invocations, Confidence: evidence.Fact},
		},
	}}
}
