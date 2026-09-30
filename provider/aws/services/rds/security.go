package rds

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// NewDBSecurityCommandExecutor runs `ctl aws rds security <identifier>`: the
// same definition fetch as `ctl aws rds def`, evaluated against a small set
// of deterministic security rules (never AI-generated — see cloudctl/
// security's package doc).
func NewDBSecurityCommandExecutor(cfg aws.Config, identifier string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: dbSecurityFetcher{cfg: cfg, identifier: identifier},
		Viewer:  security.Viewer,
	}
}

type dbSecurityFetcher struct {
	cfg        aws.Config
	identifier string
}

func (f dbSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	def, err := NewDBDefinitionCommandExecutor(f.cfg, f.identifier).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.identifier, Findings: dbSecurityFindings(def)}, nil
}

const ruleDBPubliclyAccessible = "rds-publicly-accessible"

// dbSecurityFindings flags an RDS instance/cluster configured as publicly
// accessible — a well-known, high-severity misconfiguration, directly
// readable from the same DescribeDBInstances/DescribeDBClusters data
// `ctl aws rds def` already fetches.
func dbSecurityFindings(def *dbDefinition) []security.Finding {
	if def == nil || def.publiclyAccessible == nil || !*def.publiclyAccessible {
		return nil
	}
	id := derefStr(def.identifier)
	return []security.Finding{{
		Rule:        ruleDBPubliclyAccessible,
		Severity:    security.High,
		ResourceID:  id,
		Description: "This RDS instance/cluster is configured as publicly accessible, meaning it has (or can be assigned) a public IP address reachable from the internet.",
		Remediation: "Set PubliclyAccessible to false and access the database through a bastion host, VPN, or VPC peering instead.",
		Evidence: []evidence.Evidence{
			{Source: "rds:DescribeDBInstances", ResourceID: id, Field: "PubliclyAccessible", Value: true, Confidence: evidence.Fact},
		},
	}}
}
