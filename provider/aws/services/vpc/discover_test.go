package vpc

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestDiscover_HappyPath(t *testing.T) {
	client := &fakeVPCClient{vpcsOut: &ec2.DescribeVpcsOutput{Vpcs: []types.Vpc{
		{VpcId: aws.String("vpc-1")},
		{VpcId: aws.String("vpc-2")},
	}}}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	for _, r := range resources {
		if r.Type != "vpc:vpc" {
			t.Errorf("expected type vpc:vpc, got %q", r.Type)
		}
	}
}

func TestDiscover_EmptyIsNotAnError(t *testing.T) {
	client := &fakeVPCClient{vpcsOut: &ec2.DescribeVpcsOutput{}}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("expected zero VPCs to not be an error for discovery, got %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestDiscover_APIError(t *testing.T) {
	client := &fakeVPCClient{vpcsErr: errors.New("boom")}

	_, err := Discover(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error from a failing DescribeVpcs call, got nil")
	}
}
