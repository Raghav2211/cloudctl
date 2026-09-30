package dynamodb

import (
	"cloudctl/evidence"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateTableList runs the same fetch as `ctl aws dynamodb ls`, for
// use by the investigation agent (`ctl investigate`).
func InvestigateTableList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := NewTableListCommandExecutor(cfg)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateTableDef runs the same fetch as `ctl aws dynamodb def`.
func InvestigateTableDef(ctx context.Context, cfg aws.Config, tableName string) ([]evidence.Evidence, string, error) {
	exec := NewTableDefinitionCommandExecutor(cfg, tableName)
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return tableDefinitionEvidence(def), viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}
