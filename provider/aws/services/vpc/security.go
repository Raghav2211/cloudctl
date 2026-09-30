package vpc

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// NewVPCSecurityCommandExecutor runs `ctl aws vpc security <id>`: the same
// definition fetch as `ctl aws vpc def`, evaluated against a small set of
// deterministic security rules (never AI-generated — see cloudctl/
// security's package doc).
func NewVPCSecurityCommandExecutor(cfg aws.Config, vpcID string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: vpcSecurityFetcher{cfg: cfg, vpcID: vpcID},
		Viewer:  security.Viewer,
	}
}

type vpcSecurityFetcher struct {
	cfg   aws.Config
	vpcID string
}

func (f vpcSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	def, err := NewVPCDefinitionCommandExecutor(f.cfg, f.vpcID).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.vpcID, Findings: vpcSecurityFindings(def)}, nil
}

const ruleVPCDefaultInUse = "vpc-default-vpc-in-use"

// vpcSecurityFindings flags the AWS-created default VPC — a well-known
// AWS Well-Architected recommendation against using it for production
// workloads (its default security group and routing are broader than a
// deliberately-designed VPC would be) — directly readable from the same
// DescribeVpcs data `ctl aws vpc def` already fetches.
func vpcSecurityFindings(def *vpcDefinition) []security.Finding {
	if def == nil || !def.isDefault {
		return nil
	}
	id := derefStr(def.id)
	return []security.Finding{{
		Rule:        ruleVPCDefaultInUse,
		Severity:    security.Low,
		ResourceID:  id,
		Description: "This is the AWS-created default VPC. Default VPCs are commonly recommended against for production workloads.",
		Remediation: "Migrate production workloads to a purpose-built VPC with deliberately scoped subnets, routing, and security groups; consider removing the default VPC if unused.",
		Evidence: []evidence.Evidence{
			{Source: "ec2:DescribeVpcs", ResourceID: id, Field: "IsDefault", Value: true, Confidence: evidence.Fact},
		},
	}}
}
