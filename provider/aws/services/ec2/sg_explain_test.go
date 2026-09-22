package ec2

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeDescribeSecurityGroupsClient struct {
	output *ec2.DescribeSecurityGroupsOutput
	err    error
}

func (f fakeDescribeSecurityGroupsClient) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

func testSecurityGroup() types.SecurityGroup {
	return types.SecurityGroup{
		GroupId:     aws.String("sg-0123456789abcdef0"),
		GroupName:   aws.String("payments-service-sg"),
		Description: aws.String("Payments service security group"),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(443),
				ToPort:     aws.Int32(443),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("public HTTPS")}},
			},
		},
		IpPermissionsEgress: []types.IpPermission{
			{
				IpProtocol: aws.String("-1"),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
			},
		},
	}
}

// TestSGExplainFetcher_Fetch_EndToEnd runs the full Fetch (not just the pure
// evidence-building helper), the exact kind of coverage that was missing for
// bucketConfigurationFetcher and let a real concurrency bug reach production
// (ADR-011). This fetcher has no concurrency of its own, but this still
// verifies the whole chain — client call, model construction, evidence
// building, AI wiring — behaves correctly together, not just in isolation.
func TestSGExplainFetcher_Fetch_EndToEnd(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	f := sgExplainFetcher{
		client: fakeDescribeSecurityGroupsClient{output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []types.SecurityGroup{testSecurityGroup()},
		}},
		sgId: "sg-0123456789abcdef0",
	}

	explanation, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if explanation.sgId == nil || *explanation.sgId != "sg-0123456789abcdef0" {
		t.Errorf("expected sgId to be set, got %v", explanation.sgId)
	}
	if len(explanation.ingressRules) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(explanation.ingressRules))
	}
	if len(explanation.egressRules) != 1 {
		t.Fatalf("expected 1 egress rule, got %d", len(explanation.egressRules))
	}
	if explanation.aiSummary != "" {
		t.Errorf("expected no AI summary with Ollama unreachable, got %q", explanation.aiSummary)
	}
	if explanation.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to explain why the summary is missing")
	}

	// Must render without panicking regardless of AI availability.
	view := sgExplainViewer(explanation, nil)
	view.View()
}

func TestSGExplainFetcher_Fetch_NotFound(t *testing.T) {
	f := sgExplainFetcher{
		client: fakeDescribeSecurityGroupsClient{output: &ec2.DescribeSecurityGroupsOutput{}},
		sgId:   "sg-doesnotexist",
	}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when the security group doesn't exist, got nil")
	}
}

func TestSGExplainFetcher_Fetch_APIError(t *testing.T) {
	f := sgExplainFetcher{
		client: fakeDescribeSecurityGroupsClient{err: errors.New("boom")},
		sgId:   "sg-0123456789abcdef0",
	}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeSecurityGroups call, got nil")
	}
}

func TestSecurityGroupEvidence_BuildsFactsFromRules(t *testing.T) {
	sg := testSecurityGroup()
	ingress := newSecurityIngressRules("sg-1", "my-sg", "desc", sg.IpPermissions)
	egress := newSecurityEgressRules("sg-1", "my-sg", "desc", sg.IpPermissionsEgress)

	facts := securityGroupEvidence("sg-1", "my-sg", "desc", ingress, egress)

	// 1 GroupName + 1 Description + 1 ingress + 1 egress
	if len(facts) != 4 {
		t.Fatalf("expected 4 facts, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Confidence != "FACT" {
			t.Errorf("expected all evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
	}
}
