package services

import (
	"cloudctl/cost"
	"cloudctl/executor"
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/costexplorer"
	"cloudctl/provider/aws/services/ec2"
	"cloudctl/provider/aws/services/lambda"
	"cloudctl/provider/aws/services/rds"
	"context"
	"fmt"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
)

type CostCmd struct {
	globals.AWSCLIFlag
	Days int32 `default:"30" help:"How many days back to analyze, for the Cost Explorer path"`
}

// Run tries AWS Cost Explorer first (real spend by service) and only falls
// back to a deterministic per-resource idle scan if that specific call is
// denied for lack of ce:GetCostAndUsage permission — any other error
// propagates normally, since a fallback should never mask a real problem
// (see ADR-0023).
func (cmd *CostCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	exec := &executor.CommandExecutor[*cost.Report]{
		Fetcher: costFetcher{cfg: *session, days: cmd.Days},
		Viewer:  cost.Viewer,
	}
	return exec.Execute(ctx)
}

type costFetcher struct {
	cfg  awssdk.Config
	days int32
}

func (f costFetcher) Fetch(ctx context.Context) (*cost.Report, error) {
	summary, err := costexplorer.NewSummaryFetcher(f.cfg, f.days).Fetch(ctx)
	if err == nil {
		return &cost.Report{Mode: cost.ModeCostExplorer, Summary: summary}, nil
	}
	if !costexplorer.IsAccessDenied(err) {
		return nil, err
	}

	findings, scanned, scanErr := scanIdleResources(ctx, f.cfg)
	if scanErr != nil {
		return nil, scanErr
	}
	return &cost.Report{
		Mode:           cost.ModeIdleScan,
		Findings:       findings,
		Scanned:        scanned,
		FallbackReason: fmt.Sprintf("Cost Explorer access denied (%v) — showing idle-resource findings across EC2, RDS, and Lambda instead.", err),
	}, nil
}

// idleScanResult is one service's contribution to a full idle-resource scan.
type idleScanResult struct {
	findings []cost.Finding
	scanned  int
	err      error
}

// scanIdleResources runs each service's idle-resource scan concurrently.
func scanIdleResources(ctx context.Context, cfg awssdk.Config) ([]cost.Finding, int, error) {
	results := make([]idleScanResult, 3)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		f, n, e := ec2.InvestigateIdleInstances(ctx, cfg)
		results[0] = idleScanResult{f, n, e}
	}()
	go func() {
		defer wg.Done()
		f, n, e := rds.InvestigateIdleInstances(ctx, cfg)
		results[1] = idleScanResult{f, n, e}
	}()
	go func() {
		defer wg.Done()
		f, n, e := lambda.InvestigateIdleFunctions(ctx, cfg)
		results[2] = idleScanResult{f, n, e}
	}()
	wg.Wait()

	return aggregateIdleScanResults(results)
}

// aggregateIdleScanResults combines every service's scan result. A single
// service's scan failing (e.g. it lacks a different permission) doesn't
// fail the whole report — its findings are just omitted — unless every
// service fails, in which case the first error is surfaced rather than
// silently reporting zero findings as if the account were clean.
func aggregateIdleScanResults(results []idleScanResult) ([]cost.Finding, int, error) {
	var findings []cost.Finding
	var scanned int
	succeeded := false
	for _, r := range results {
		if r.err != nil {
			continue
		}
		succeeded = true
		findings = append(findings, r.findings...)
		scanned += r.scanned
	}
	if !succeeded && len(results) > 0 {
		return nil, 0, results[0].err
	}
	return findings, scanned, nil
}
