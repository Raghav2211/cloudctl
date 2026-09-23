package vpc

import "github.com/aws/aws-sdk-go-v2/service/ec2/types"

type vpcSummary struct {
	id        *string
	cidr      *string
	isDefault bool
	state     *string
}

type vpcListOutput struct {
	vpcs []*vpcSummary
}

// subnetInfo is one subnet within a vpcDefinition. isPublic is a
// deterministic classification (evidence.Inference, not an AI judgment
// call) derived from the subnet's route table: public iff that route table
// has a route to an internet gateway.
type subnetInfo struct {
	id               *string
	cidr             *string
	az               *string
	isPublic         bool
	availableIPCount *int32
}

type natGatewayInfo struct {
	id       *string
	subnetID *string
	state    *string
}

type internetGatewayInfo struct {
	id *string
}

// vpcDefinition is `ctl aws vpc def <vpc-id>`'s output: the deterministic
// VPC/subnet/gateway topology plus a Hypothesis-grade AI narration of it.
// aiSummary is additive, never a replacement for the raw fields below — if
// empty, aiSummaryUnavailable explains why (ADR-010), mirroring every other
// *Definition type in this codebase.
type vpcDefinition struct {
	id        *string
	cidr      *string
	isDefault bool
	state     *string

	subnets          []*subnetInfo
	natGateways      []*natGatewayInfo
	internetGateways []*internetGatewayInfo

	aiSummary            string
	aiSummaryUnavailable string
}

func newVPCSummary(v types.Vpc) *vpcSummary {
	state := string(v.State)
	return &vpcSummary{
		id:        v.VpcId,
		cidr:      v.CidrBlock,
		isDefault: v.IsDefault != nil && *v.IsDefault,
		state:     &state,
	}
}

func newVPCDefinition(v types.Vpc) *vpcDefinition {
	state := string(v.State)
	return &vpcDefinition{
		id:        v.VpcId,
		cidr:      v.CidrBlock,
		isDefault: v.IsDefault != nil && *v.IsDefault,
		state:     &state,
	}
}

func (def *vpcDefinition) SetSubnets(subnets []*subnetInfo) *vpcDefinition {
	def.subnets = subnets
	return def
}

func (def *vpcDefinition) SetNatGateways(gateways []*natGatewayInfo) *vpcDefinition {
	def.natGateways = gateways
	return def
}

func (def *vpcDefinition) SetInternetGateways(gateways []*internetGatewayInfo) *vpcDefinition {
	def.internetGateways = gateways
	return def
}

func (def *vpcDefinition) SetAISummary(summary string) *vpcDefinition {
	def.aiSummary = summary
	return def
}

func (def *vpcDefinition) SetAISummaryUnavailable(reason string) *vpcDefinition {
	def.aiSummaryUnavailable = reason
	return def
}
