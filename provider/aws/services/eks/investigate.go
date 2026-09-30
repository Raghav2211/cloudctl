package eks

import (
	"cloudctl/evidence"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateClusterList runs the same fetch as `ctl aws eks ls`, for use
// by the investigation agent (`ctl investigate`).
func InvestigateClusterList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := NewClusterListCommandExecutor(cfg)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateClusterDef runs the same fetch as `ctl aws eks def`.
func InvestigateClusterDef(ctx context.Context, cfg aws.Config, clusterName string) ([]evidence.Evidence, string, error) {
	exec := NewClusterDefinitionCommandExecutor(cfg, clusterName)
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return clusterDefinitionEvidence(def), viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}
