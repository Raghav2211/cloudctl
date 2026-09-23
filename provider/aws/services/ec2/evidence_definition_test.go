package ec2

import (
	"cloudctl/provider/aws"
	"cloudctl/viewer"
	"errors"
	"testing"
)

// TestInstanceDefinitionEvidence_FullyPopulated checks that every section
// (summary, detail, volumes, rules, network interfaces) contributes facts
// when its data is present.
func TestInstanceDefinitionEvidence_FullyPopulated(t *testing.T) {
	id := "i-0123456789abcdef0"
	typee, state, az, vpc, subnet, pubIp, privIp, iamArn := "t3.micro", "running", "eu-west-1a", "vpc-1", "subnet-1", "1.2.3.4", "10.0.0.1", "arn:aws:iam::123:role/x"
	def := newInstanceDefinition()
	def.SetInstanceSummary(&instanceSummary{
		id: &id, typee: &typee, state: &state, az: &az, vpcId: &vpc, subnetId: &subnet,
		publicIp: &pubIp, privateIp: &privIp, iamroleArn: &iamArn,
	})
	platform, ami, monitor, os := "linux", "ami-1", "enabled", "Linux/UNIX"
	def.SetInstanceDetail(&instanceDetail{platform: &platform, amiId: &ami, monitor: &monitor, osdetails: &os})

	size := int32(8)
	encrypted := true
	volState := "in-use"
	def.SetVolumeSummary(newInstanceVolumeSummary([]*instanceVolume{
		{size: &size, isEncrypt: &encrypted, state: &volState},
	}, nil))

	protocol, portRange, source, desc := "TCP", "443", "0.0.0.0/0", "https"
	def.SetInstanceIngressEgressRuleSummary(&instanceIngressEgressRuleSummary{
		ingressRules: []*ingressRule{{protocol: &protocol, portRange: &portRange, source: &source, desc: &desc}},
		egressRules:  []*egressRule{{protocol: &protocol, portRange: &portRange, source: &source, desc: &desc}},
	})

	eniId, eniPub, eniPriv := "eni-1", "1.2.3.4", "10.0.0.1"
	def.SetNetworkInterfaces([]*instanceNetworkinterface{{id: &eniId, publicIpv4Add: &eniPub, privateIpv4Add: &eniPriv}})

	facts := instanceDefinitionEvidence(def)

	// 8 summary + 4 detail + 1 volume + 1 ingress + 1 egress + 1 eni
	if len(facts) != 16 {
		t.Fatalf("expected 16 facts, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Confidence != "FACT" {
			t.Errorf("expected all evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
	}
}

// TestInstanceDefinitionEvidence_DegradesGracefully mirrors the concern
// behind ADR-011: sub-fetches (volumes, rules) run concurrently and may
// independently fail or be skipped (e.g. no BlockDeviceMappings). Evidence
// building must never assume full population.
func TestInstanceDefinitionEvidence_DegradesGracefully(t *testing.T) {
	def := newInstanceDefinition()
	facts := instanceDefinitionEvidence(def)
	if len(facts) != 0 {
		t.Fatalf("expected no facts from an entirely empty instanceDefinition, got %d: %+v", len(facts), facts)
	}

	id := "i-0123456789abcdef0"
	def.SetInstanceSummary(&instanceSummary{id: &id})
	def.SetVolumeSummary(newInstanceVolumeSummary(nil, aws.NewErrorInfo(errors.New("boom"), viewer.ERROR, nil)))
	def.SetInstanceIngressEgressRuleSummary(&instanceIngressEgressRuleSummary{apiError: aws.NewErrorInfo(errors.New("boom"), viewer.ERROR, nil)})

	facts = instanceDefinitionEvidence(def)
	for _, f := range facts {
		if f.Field == "Volume" || f.Field == "IngressRule" || f.Field == "EgressRule" {
			t.Fatalf("expected no evidence from a failed sub-fetch, got %+v", f)
		}
	}
}
