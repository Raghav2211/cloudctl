package vpc

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// vpcAPI is the minimal client capability this package needs, letting tests
// substitute a fake instead of a real *ec2.Client (ADR-007).
type vpcAPI interface {
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
	DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
	DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
}

type vpcListFetcher struct {
	client vpcAPI
}

type vpcDefinitionFetcher struct {
	client vpcAPI
	vpcID  string
}

func vpcIDFilter(vpcID string) []types.Filter {
	return []types.Filter{{Name: ptrStr("vpc-id"), Values: []string{vpcID}}}
}

func ptrStr(s string) *string { return &s }

func (f vpcListFetcher) Fetch(ctx context.Context) (*vpcListOutput, error) {
	var vpcs []*vpcSummary
	paginator := ec2.NewDescribeVpcsPaginator(f.client, &ec2.DescribeVpcsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, v := range page.Vpcs {
			vpcs = append(vpcs, newVPCSummary(v))
		}
	}
	if len(vpcs) == 0 {
		return nil, ctlaws.NewErrorInfo(NoVPCFound(), viewer.INFO, nil)
	}
	return &vpcListOutput{vpcs: vpcs}, nil
}

// Fetch gathers a VPC's own attributes plus its subnets, route tables (used
// only to classify subnets — not rendered on their own), NAT gateways, and
// internet gateways, then narrates the topology via Summarize. Mirrors
// bucketConfigurationFetcher/instanceDefinitionFetcher: AI narration is
// additive (ADR-010) — a failed or unreachable Summarize call never fails
// Fetch itself.
func (f vpcDefinitionFetcher) Fetch(ctx context.Context) (*vpcDefinition, error) {
	vpcOut, err := f.client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{f.vpcID}})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if len(vpcOut.Vpcs) == 0 {
		return nil, ctlaws.NewErrorInfo(VPCNotFound(f.vpcID), viewer.INFO, nil)
	}
	def := newVPCDefinition(vpcOut.Vpcs[0])

	subnetsOut, err := f.client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{Filters: vpcIDFilter(f.vpcID)})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	routeTablesOut, err := f.client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{Filters: vpcIDFilter(f.vpcID)})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	def.SetSubnets(classifySubnets(subnetsOut.Subnets, routeTablesOut.RouteTables))

	natOut, err := f.client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{Filter: vpcIDFilter(f.vpcID)})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	nats := make([]*natGatewayInfo, 0, len(natOut.NatGateways))
	for _, n := range natOut.NatGateways {
		state := string(n.State)
		nats = append(nats, &natGatewayInfo{id: n.NatGatewayId, subnetID: n.SubnetId, state: &state})
	}
	def.SetNatGateways(nats)

	igwOut, err := f.client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{{Name: ptrStr("attachment.vpc-id"), Values: []string{f.vpcID}}},
	})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	igws := make([]*internetGatewayInfo, 0, len(igwOut.InternetGateways))
	for _, g := range igwOut.InternetGateways {
		igws = append(igws, &internetGatewayInfo{id: g.InternetGatewayId})
	}
	def.SetInternetGateways(igws)

	def.applyAISummary(ctx, ai.NewClientFromEnv(), vpcDefinitionEvidence(def))

	return def, nil
}

// applyAISummary sets aiSummary or aiSummaryUnavailable from the given
// evidence, never returning an error (ADR-010).
func (def *vpcDefinition) applyAISummary(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		def.SetAISummaryUnavailable("no evidence could be gathered")
		return
	}
	summary, err := viewer.WithSpinner("Generating AI summary...", func() (string, error) {
		return client.Summarize(ctx, facts)
	})
	if err != nil {
		def.SetAISummaryUnavailable(err.Error())
		return
	}
	def.SetAISummary(summary)
}
