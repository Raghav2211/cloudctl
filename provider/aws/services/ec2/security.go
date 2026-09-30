package ec2

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	ctltime "cloudctl/time"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
)

// NewInstanceSecurityCommandExecutor runs `ctl aws ec2 security <id>`: the
// same definition fetch as `ctl aws ec2 def`, evaluated against a small set
// of deterministic security rules (never AI-generated — see cloudctl/
// security's package doc). Bypasses NewInstanceDescribeCommandExecutor
// (which needs a CLI AWSCLIFlag) and builds the same fetcher directly,
// mirroring investigate.go's InvestigateInstanceDef.
func NewInstanceSecurityCommandExecutor(cfg aws.Config, instanceID string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: instanceSecurityFetcher{cfg: cfg, instanceID: instanceID},
		Viewer:  security.Viewer,
	}
}

type instanceSecurityFetcher struct {
	cfg        aws.Config
	instanceID string
}

func (f instanceSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	fetcher := &instanceDefinitionFetcher{
		client: awsec2.NewFromConfig(f.cfg),
		id:     &f.instanceID,
		tz:     ctltime.GetTZ("UTC"),
	}
	def, err := fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.instanceID, Findings: instanceSecurityFindings(def)}, nil
}

const ruleEC2PublicOpenIngress = "ec2-public-instance-open-ingress"

// openIngressCIDR is the classic "wide open" ingress source — a security
// group rule allowing traffic from anywhere.
const openIngressCIDR = "0.0.0.0/0"

// instanceSecurityFindings flags an instance that has a public IP and at
// least one security group ingress rule open to the entire internet —
// directly readable from the same DescribeInstances/security-group data
// `ctl aws ec2 def` already fetches.
func instanceSecurityFindings(def *instanceDefinition) []security.Finding {
	if def == nil || def.summary == nil || def.ruleSummary == nil {
		return nil
	}
	if def.summary.publicIp == nil || *def.summary.publicIp == "" {
		return nil
	}

	var openRules []*ingressRule
	for _, r := range def.ruleSummary.ingressRules {
		if r.source != nil && *r.source == openIngressCIDR {
			openRules = append(openRules, r)
		}
	}
	if len(openRules) == 0 {
		return nil
	}

	id := derefStr(def.summary.id)
	facts := []evidence.Evidence{
		{Source: "ec2:DescribeInstances", ResourceID: id, Field: "PublicIP", Value: *def.summary.publicIp, Confidence: evidence.Fact},
	}
	for _, r := range openRules {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: id, Field: "OpenIngress",
			Value:      fmt.Sprintf("port=%s protocol=%s source=%s", derefStr(r.portRange), derefStr(r.protocol), derefStr(r.source)),
			Confidence: evidence.Fact,
		})
	}

	return []security.Finding{{
		Rule:        ruleEC2PublicOpenIngress,
		Severity:    security.High,
		ResourceID:  id,
		Description: fmt.Sprintf("This instance has a public IP (%s) and at least one security group rule allowing inbound traffic from 0.0.0.0/0.", *def.summary.publicIp),
		Remediation: "Restrict the security group's ingress rules to known CIDR ranges, or remove the public IP if internet access isn't required.",
		Evidence:    facts,
	}}
}
