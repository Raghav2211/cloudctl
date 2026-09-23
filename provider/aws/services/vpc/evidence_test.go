package vpc

import (
	"cloudctl/evidence"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestRouteTableIsPublic_WithInternetGatewayRoute(t *testing.T) {
	rt := types.RouteTable{
		Routes: []types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
			{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String("igw-0123456789abcdef0")},
		},
	}
	if !routeTableIsPublic(rt) {
		t.Error("expected a route table with an igw-* route to be classified public")
	}
}

func TestRouteTableIsPublic_NatGatewayOnlyIsPrivate(t *testing.T) {
	rt := types.RouteTable{
		Routes: []types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
			{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String("nat-0123456789abcdef0")},
		},
	}
	if routeTableIsPublic(rt) {
		t.Error("expected a route table with only a NAT gateway route to be classified private")
	}
}

func TestRouteTableIsPublic_LocalOnlyIsPrivate(t *testing.T) {
	rt := types.RouteTable{
		Routes: []types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16"), GatewayId: aws.String("local")},
		},
	}
	if routeTableIsPublic(rt) {
		t.Error("expected a route table with only a local route to be classified private")
	}
}

// TestClassifySubnets_ExplicitAssociation confirms a subnet explicitly
// associated with a public route table is classified public, and one
// associated with a private route table is classified private.
func TestClassifySubnets_ExplicitAssociation(t *testing.T) {
	publicRT := types.RouteTable{
		RouteTableId: aws.String("rtb-public"),
		Routes:       []types.Route{{GatewayId: aws.String("igw-0123456789abcdef0")}},
		Associations: []types.RouteTableAssociation{{SubnetId: aws.String("subnet-public")}},
	}
	privateRT := types.RouteTable{
		RouteTableId: aws.String("rtb-private"),
		Routes:       []types.Route{{NatGatewayId: aws.String("nat-0123456789abcdef0")}},
		Associations: []types.RouteTableAssociation{{SubnetId: aws.String("subnet-private")}},
	}
	subnets := []types.Subnet{
		{SubnetId: aws.String("subnet-public"), CidrBlock: aws.String("10.0.1.0/24")},
		{SubnetId: aws.String("subnet-private"), CidrBlock: aws.String("10.0.2.0/24")},
	}

	infos := classifySubnets(subnets, []types.RouteTable{publicRT, privateRT})
	if len(infos) != 2 {
		t.Fatalf("expected 2 subnet infos, got %d", len(infos))
	}
	byID := map[string]*subnetInfo{}
	for _, i := range infos {
		byID[derefStr(i.id)] = i
	}
	if !byID["subnet-public"].isPublic {
		t.Error("expected subnet-public to be classified public")
	}
	if byID["subnet-private"].isPublic {
		t.Error("expected subnet-private to be classified private")
	}
}

// TestClassifySubnets_FallsBackToMainRouteTable confirms a subnet with no
// explicit route table association inherits the VPC's main (implicit)
// route table, matching how AWS itself resolves routing.
func TestClassifySubnets_FallsBackToMainRouteTable(t *testing.T) {
	mainRT := types.RouteTable{
		RouteTableId: aws.String("rtb-main"),
		Routes:       []types.Route{{GatewayId: aws.String("igw-0123456789abcdef0")}},
		Associations: []types.RouteTableAssociation{{Main: aws.Bool(true)}},
	}
	subnets := []types.Subnet{
		{SubnetId: aws.String("subnet-unassociated"), CidrBlock: aws.String("10.0.3.0/24")},
	}

	infos := classifySubnets(subnets, []types.RouteTable{mainRT})
	if len(infos) != 1 {
		t.Fatalf("expected 1 subnet info, got %d", len(infos))
	}
	if !infos[0].isPublic {
		t.Error("expected the unassociated subnet to inherit the public main route table")
	}
}

func TestVPCDefinitionEvidence_PublicPrivateSplitIsInferenceTagged(t *testing.T) {
	def := &vpcDefinition{
		id:    aws.String("vpc-0123456789abcdef0"),
		cidr:  aws.String("10.0.0.0/16"),
		state: aws.String("available"),
		subnets: []*subnetInfo{
			{id: aws.String("subnet-a"), cidr: aws.String("10.0.1.0/24"), isPublic: true},
			{id: aws.String("subnet-b"), cidr: aws.String("10.0.2.0/24"), isPublic: false},
		},
	}

	facts := vpcDefinitionEvidence(def)
	var sawInference, sawFact bool
	for _, f := range facts {
		if f.Field == "SubnetSplit" {
			if f.Confidence != evidence.Inference {
				t.Errorf("expected SubnetSplit to be Inference-tagged, got %q", f.Confidence)
			}
			sawInference = true
		}
		if f.Field == "CIDR" {
			if f.Confidence != evidence.Fact {
				t.Errorf("expected CIDR to be Fact-tagged, got %q", f.Confidence)
			}
			sawFact = true
		}
	}
	if !sawInference || !sawFact {
		t.Fatal("expected both an Inference-tagged and a Fact-tagged evidence item")
	}
}
