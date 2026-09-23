package vpc

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// fakeVPCClient implements vpcAPI with canned responses, letting tests
// substitute it for a real *ec2.Client (ADR-007).
type fakeVPCClient struct {
	vpcsOut        *ec2.DescribeVpcsOutput
	vpcsErr        error
	subnetsOut     *ec2.DescribeSubnetsOutput
	subnetsErr     error
	routeTablesOut *ec2.DescribeRouteTablesOutput
	routeTablesErr error
	natOut         *ec2.DescribeNatGatewaysOutput
	natErr         error
	igwOut         *ec2.DescribeInternetGatewaysOutput
	igwErr         error
}

func (f *fakeVPCClient) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if f.vpcsErr != nil {
		return nil, f.vpcsErr
	}
	if f.vpcsOut == nil {
		return &ec2.DescribeVpcsOutput{}, nil
	}
	return f.vpcsOut, nil
}

func (f *fakeVPCClient) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if f.subnetsErr != nil {
		return nil, f.subnetsErr
	}
	if f.subnetsOut == nil {
		return &ec2.DescribeSubnetsOutput{}, nil
	}
	return f.subnetsOut, nil
}

func (f *fakeVPCClient) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	if f.routeTablesErr != nil {
		return nil, f.routeTablesErr
	}
	if f.routeTablesOut == nil {
		return &ec2.DescribeRouteTablesOutput{}, nil
	}
	return f.routeTablesOut, nil
}

func (f *fakeVPCClient) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	if f.natErr != nil {
		return nil, f.natErr
	}
	if f.natOut == nil {
		return &ec2.DescribeNatGatewaysOutput{}, nil
	}
	return f.natOut, nil
}

func (f *fakeVPCClient) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	if f.igwErr != nil {
		return nil, f.igwErr
	}
	if f.igwOut == nil {
		return &ec2.DescribeInternetGatewaysOutput{}, nil
	}
	return f.igwOut, nil
}

func testVPC(id string) types.Vpc {
	return types.Vpc{
		VpcId:     aws.String(id),
		CidrBlock: aws.String("10.0.0.0/16"),
		IsDefault: aws.Bool(false),
		State:     types.VpcStateAvailable,
	}
}

func TestVPCListFetcher_Fetch_HappyPath(t *testing.T) {
	client := &fakeVPCClient{vpcsOut: &ec2.DescribeVpcsOutput{Vpcs: []types.Vpc{testVPC("vpc-1"), testVPC("vpc-2")}}}
	f := vpcListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.vpcs) != 2 {
		t.Fatalf("expected 2 VPCs, got %d", len(out.vpcs))
	}
}

func TestVPCListFetcher_Fetch_EmptyResult(t *testing.T) {
	f := vpcListFetcher{client: &fakeVPCClient{}}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no VPCs are found, got nil")
	}
}

func TestVPCListFetcher_Fetch_APIError(t *testing.T) {
	f := vpcListFetcher{client: &fakeVPCClient{vpcsErr: errors.New("boom")}}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeVpcs call, got nil")
	}
}

func TestVPCDefinitionFetcher_Fetch_EndToEnd(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	client := &fakeVPCClient{
		vpcsOut: &ec2.DescribeVpcsOutput{Vpcs: []types.Vpc{testVPC("vpc-1")}},
		subnetsOut: &ec2.DescribeSubnetsOutput{Subnets: []types.Subnet{
			{SubnetId: aws.String("subnet-public"), CidrBlock: aws.String("10.0.1.0/24"), AvailabilityZone: aws.String("eu-west-1a")},
		}},
		routeTablesOut: &ec2.DescribeRouteTablesOutput{RouteTables: []types.RouteTable{
			{
				RouteTableId: aws.String("rtb-1"),
				Routes:       []types.Route{{GatewayId: aws.String("igw-0123456789abcdef0")}},
				Associations: []types.RouteTableAssociation{{SubnetId: aws.String("subnet-public")}},
			},
		}},
		igwOut: &ec2.DescribeInternetGatewaysOutput{InternetGateways: []types.InternetGateway{
			{InternetGatewayId: aws.String("igw-0123456789abcdef0")},
		}},
	}
	f := vpcDefinitionFetcher{client: client, vpcID: "vpc-1"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.subnets) != 1 {
		t.Fatalf("expected 1 subnet, got %d", len(def.subnets))
	}
	if !def.subnets[0].isPublic {
		t.Error("expected the subnet to be classified public")
	}
	if len(def.internetGateways) != 1 {
		t.Errorf("expected 1 internet gateway, got %d", len(def.internetGateways))
	}

	vpcDefinitionViewer(def, nil).View() // must not panic
}

func TestVPCDefinitionFetcher_Fetch_NotFound(t *testing.T) {
	f := vpcDefinitionFetcher{client: &fakeVPCClient{}, vpcID: "vpc-does-not-exist"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when the VPC doesn't exist, got nil")
	}
}

func TestVPCDefinitionFetcher_Fetch_APIError(t *testing.T) {
	f := vpcDefinitionFetcher{client: &fakeVPCClient{vpcsErr: errors.New("boom")}, vpcID: "vpc-1"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeVpcs call, got nil")
	}
}
