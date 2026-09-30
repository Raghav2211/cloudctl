package eks

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// NewClusterSecurityCommandExecutor runs `ctl aws eks security <cluster>`:
// the same definition fetch as `ctl aws eks def`, evaluated against a small
// set of deterministic security rules (never AI-generated — see cloudctl/
// security's package doc).
func NewClusterSecurityCommandExecutor(cfg aws.Config, clusterName string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: clusterSecurityFetcher{cfg: cfg, clusterName: clusterName},
		Viewer:  security.Viewer,
	}
}

type clusterSecurityFetcher struct {
	cfg         aws.Config
	clusterName string
}

func (f clusterSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	def, err := NewClusterDefinitionCommandExecutor(f.cfg, f.clusterName).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.clusterName, Findings: clusterSecurityFindings(def)}, nil
}

const ruleEKSPublicEndpoint = "eks-public-endpoint-access"

// clusterSecurityFindings flags an EKS cluster whose API server endpoint is
// reachable from the public internet — directly readable from the same
// DescribeCluster data `ctl aws eks def` already fetches.
func clusterSecurityFindings(def *clusterDefinition) []security.Finding {
	if def == nil || !def.endpointPublicAccess {
		return nil
	}
	id := derefStr(def.name)
	cidrs := "0.0.0.0/0 (unrestricted)"
	if len(def.publicAccessCidrs) > 0 {
		cidrs = strings.Join(def.publicAccessCidrs, ", ")
	}
	return []security.Finding{{
		Rule:        ruleEKSPublicEndpoint,
		Severity:    security.High,
		ResourceID:  id,
		Description: "This EKS cluster's API server endpoint has public access enabled, from: " + cidrs + ".",
		Remediation: "Disable public endpoint access (or restrict publicAccessCidrs to known, narrow CIDR ranges) and rely on private access from within the VPC instead.",
		Evidence: []evidence.Evidence{
			{Source: "eks:DescribeCluster", ResourceID: id, Field: "EndpointPublicAccess", Value: true, Confidence: evidence.Fact},
			{Source: "eks:DescribeCluster", ResourceID: id, Field: "PublicAccessCidrs", Value: cidrs, Confidence: evidence.Fact},
		},
	}}
}
