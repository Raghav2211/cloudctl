package vpc

import (
	"cloudctl/evidence"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateVPCList runs the same fetch as `ctl aws vpc ls`, for use by
// the investigation agent (`ctl investigate`).
func InvestigateVPCList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := NewVPCListCommandExecutor(cfg)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateVPCDef runs the same fetch as `ctl aws vpc def`.
func InvestigateVPCDef(ctx context.Context, cfg aws.Config, vpcID string) ([]evidence.Evidence, string, error) {
	exec := NewVPCDefinitionCommandExecutor(cfg, vpcID)
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return vpcDefinitionEvidence(def), viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}
