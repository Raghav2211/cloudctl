package ec2

import (
	"cloudctl/evidence"
	"fmt"
)

// securityGroupEvidence converts a security group's already-parsed rules
// into Fact-tagged evidence for Summarize.
func securityGroupEvidence(sgId, sgName, description string, ingress []*ingressRule, egress []*egressRule) []evidence.Evidence {
	facts := []evidence.Evidence{
		{Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "GroupName", Value: sgName, Confidence: evidence.Fact},
	}
	if description != "" {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "Description",
			Value: description, Confidence: evidence.Fact,
		})
	}
	for _, r := range ingress {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "IngressRule",
			Value:      fmt.Sprintf("%s/%s from %s (%s)", derefStr(r.protocol), derefStr(r.portRange), derefStr(r.source), derefStr(r.desc)),
			Confidence: evidence.Fact,
		})
	}
	for _, r := range egress {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "EgressRule",
			Value:      fmt.Sprintf("%s/%s to %s (%s)", derefStr(r.protocol), derefStr(r.portRange), derefStr(r.source), derefStr(r.desc)),
			Confidence: evidence.Fact,
		})
	}
	return facts
}

func derefStr(s *string) string {
	if s == nil {
		return NO_VALUE
	}
	return *s
}
