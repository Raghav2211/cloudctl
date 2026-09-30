package rds

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

func TestDiscover_HappyPath(t *testing.T) {
	client := &fakeRDSClient{
		instOut:    &rds.DescribeDBInstancesOutput{DBInstances: []types.DBInstance{testDBInstance("orders-db")}},
		clusterOut: &rds.DescribeDBClustersOutput{DBClusters: []types.DBCluster{{DBClusterIdentifier: aws.String("orders-cluster")}}},
	}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources (1 instance + 1 cluster), got %d", len(resources))
	}

	var sawInstance, sawCluster bool
	for _, r := range resources {
		switch r.Type {
		case "rds:instance":
			sawInstance = true
		case "rds:cluster":
			sawCluster = true
		default:
			t.Errorf("unexpected resource type %q", r.Type)
		}
	}
	if !sawInstance || !sawCluster {
		t.Errorf("expected both an instance and a cluster resource, sawInstance=%v sawCluster=%v", sawInstance, sawCluster)
	}
}

func TestDiscover_EmptyIsNotAnError(t *testing.T) {
	client := &fakeRDSClient{
		instOut:    &rds.DescribeDBInstancesOutput{},
		clusterOut: &rds.DescribeDBClustersOutput{},
	}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("expected zero instances/clusters to not be an error for discovery, got %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestDiscover_InstanceAPIError(t *testing.T) {
	client := &fakeRDSClient{instErr: errors.New("boom")}

	_, err := Discover(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error from a failing DescribeDBInstances call, got nil")
	}
}

func TestDiscover_ClusterAPIError(t *testing.T) {
	client := &fakeRDSClient{
		instOut:    &rds.DescribeDBInstancesOutput{},
		clusterErr: errors.New("boom"),
	}

	_, err := Discover(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error from a failing DescribeDBClusters call, got nil")
	}
}
