package ec2

import "testing"

func TestInstanceSecurityFindings_FlagsPublicOpenIngress(t *testing.T) {
	id := "i-abc123"
	publicIp := "203.0.113.5"
	port := "22"
	proto := "tcp"
	source := "0.0.0.0/0"

	def := &instanceDefinition{
		summary: &instanceSummary{id: &id, publicIp: &publicIp},
		ruleSummary: &instanceIngressEgressRuleSummary{
			ingressRules: []*ingressRule{{portRange: &port, protocol: &proto, source: &source}},
		},
	}

	findings := instanceSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleEC2PublicOpenIngress {
		t.Fatalf("expected 1 %s finding, got %+v", ruleEC2PublicOpenIngress, findings)
	}
}

func TestInstanceSecurityFindings_NoFindingWithoutPublicIP(t *testing.T) {
	id := "i-abc123"
	port := "22"
	proto := "tcp"
	source := "0.0.0.0/0"

	def := &instanceDefinition{
		summary: &instanceSummary{id: &id},
		ruleSummary: &instanceIngressEgressRuleSummary{
			ingressRules: []*ingressRule{{portRange: &port, protocol: &proto, source: &source}},
		},
	}

	if findings := instanceSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings without a public IP, got %+v", findings)
	}
}

func TestInstanceSecurityFindings_NoFindingWithNarrowIngress(t *testing.T) {
	id := "i-abc123"
	publicIp := "203.0.113.5"
	port := "22"
	proto := "tcp"
	source := "10.0.0.0/16"

	def := &instanceDefinition{
		summary: &instanceSummary{id: &id, publicIp: &publicIp},
		ruleSummary: &instanceIngressEgressRuleSummary{
			ingressRules: []*ingressRule{{portRange: &port, protocol: &proto, source: &source}},
		},
	}

	if findings := instanceSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a narrow-CIDR ingress rule, got %+v", findings)
	}
}
