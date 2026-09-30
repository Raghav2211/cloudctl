package lambda

import (
	"cloudctl/evidence"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateFunctionList runs the same fetch as `ctl aws lambda ls`, for
// use by the investigation agent (`ctl investigate`).
func InvestigateFunctionList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := NewFunctionListCommandExecutor(cfg)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateFunctionDef runs the same fetch as `ctl aws lambda def`.
func InvestigateFunctionDef(ctx context.Context, cfg aws.Config, functionName string) ([]evidence.Evidence, string, error) {
	exec := NewFunctionDefinitionCommandExecutor(cfg, functionName)
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return functionDefinitionEvidence(def), viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}

// InvestigateFunctionStats runs the same fetch as `ctl aws lambda stats`.
func InvestigateFunctionStats(ctx context.Context, cfg aws.Config, functionName string) ([]evidence.Evidence, string, error) {
	exec := NewFunctionStatisticsCommandExecutor(cfg, functionName)
	stats, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return functionStatisticsEvidence(stats), viewer.StructuredJSON(exec.Viewer(stats, nil)), nil
}
