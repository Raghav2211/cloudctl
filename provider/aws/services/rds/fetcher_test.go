package rds

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// fakeRDSClient implements dbAPI with canned responses, letting tests
// substitute it for a real *rds.Client (ADR-007).
type fakeRDSClient struct {
	instOut *rds.DescribeDBInstancesOutput
	instErr error

	clusterOut *rds.DescribeDBClustersOutput
	clusterErr error
}

func (f *fakeRDSClient) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	if f.instErr != nil {
		return nil, f.instErr
	}
	if f.instOut == nil {
		return &rds.DescribeDBInstancesOutput{}, nil
	}
	return f.instOut, nil
}

func (f *fakeRDSClient) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	if f.clusterErr != nil {
		return nil, f.clusterErr
	}
	if f.clusterOut == nil {
		return &rds.DescribeDBClustersOutput{}, nil
	}
	return f.clusterOut, nil
}

func testDBInstance(id string) types.DBInstance {
	return types.DBInstance{
		DBInstanceIdentifier:  aws.String(id),
		DBInstanceStatus:      aws.String("available"),
		Engine:                aws.String("postgres"),
		EngineVersion:         aws.String("16.4"),
		DBInstanceClass:       aws.String("db.t3.micro"),
		AllocatedStorage:      aws.Int32(20),
		MultiAZ:               aws.Bool(false),
		StorageEncrypted:      aws.Bool(true),
		PubliclyAccessible:    aws.Bool(false),
		BackupRetentionPeriod: aws.Int32(7),
		Endpoint:              &types.Endpoint{Address: aws.String(id + ".xxxx.eu-west-1.rds.amazonaws.com")},
	}
}

func testDBCluster(id string) types.DBCluster {
	return types.DBCluster{
		DBClusterIdentifier: aws.String(id),
		Status:              aws.String("available"),
		Engine:              aws.String("aurora-postgresql"),
		EngineVersion:       aws.String("15.4"),
		MultiAZ:             aws.Bool(true),
		StorageEncrypted:    aws.Bool(true),
		PubliclyAccessible:  aws.Bool(false),
		Endpoint:            aws.String(id + ".cluster-xxxx.eu-west-1.rds.amazonaws.com"),
	}
}

func TestDBListFetcher_Fetch_CombinesInstancesAndClusters(t *testing.T) {
	client := &fakeRDSClient{
		instOut:    &rds.DescribeDBInstancesOutput{DBInstances: []types.DBInstance{testDBInstance("orders-db")}},
		clusterOut: &rds.DescribeDBClustersOutput{DBClusters: []types.DBCluster{testDBCluster("analytics-cluster")}},
	}
	f := dbListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.items) != 2 {
		t.Fatalf("expected 1 instance + 1 cluster = 2 items, got %d", len(out.items))
	}
}

func TestDBListFetcher_Fetch_EmptyResult(t *testing.T) {
	f := dbListFetcher{client: &fakeRDSClient{}}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no instances or clusters are found, got nil")
	}
}

func TestDBListFetcher_Fetch_InstanceAPIError(t *testing.T) {
	f := dbListFetcher{client: &fakeRDSClient{instErr: errors.New("boom")}}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeDBInstances call, got nil")
	}
}

func TestDBDefinitionFetcher_Fetch_InstanceFound(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	client := &fakeRDSClient{
		instOut: &rds.DescribeDBInstancesOutput{DBInstances: []types.DBInstance{testDBInstance("orders-db")}},
	}
	f := dbDefinitionFetcher{client: client, identifier: "orders-db"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.kind != "instance" {
		t.Errorf("expected kind 'instance', got %q", def.kind)
	}
	if derefStr(def.engine) != "postgres" {
		t.Errorf("expected engine 'postgres', got %q", derefStr(def.engine))
	}

	dbDefinitionViewer(def, nil).View() // must not panic
}

// TestDBDefinitionFetcher_Fetch_FallsBackToCluster confirms an identifier
// that isn't a DB instance but is a DB cluster (e.g. an Aurora cluster
// identifier) is still found — instance and cluster are separate identifier
// namespaces sharing one `def` command.
func TestDBDefinitionFetcher_Fetch_FallsBackToCluster(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	client := &fakeRDSClient{
		instErr:    &types.DBInstanceNotFoundFault{Message: aws.String("not found")},
		clusterOut: &rds.DescribeDBClustersOutput{DBClusters: []types.DBCluster{testDBCluster("analytics-cluster")}},
	}
	f := dbDefinitionFetcher{client: client, identifier: "analytics-cluster"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.kind != "cluster" {
		t.Errorf("expected kind 'cluster', got %q", def.kind)
	}
	if !derefBool(def.multiAZ) {
		t.Error("expected MultiAZ true for the test cluster")
	}

	dbDefinitionViewer(def, nil).View() // must not panic
}

func TestDBDefinitionFetcher_Fetch_NotFoundInEither(t *testing.T) {
	client := &fakeRDSClient{
		instErr:    &types.DBInstanceNotFoundFault{Message: aws.String("not found")},
		clusterErr: &types.DBClusterNotFoundFault{Message: aws.String("not found")},
	}
	f := dbDefinitionFetcher{client: client, identifier: "does-not-exist"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when neither an instance nor a cluster matches, got nil")
	}
}

func TestDBDefinitionFetcher_Fetch_RealAPIError(t *testing.T) {
	client := &fakeRDSClient{instErr: errors.New("access denied")}
	f := dbDefinitionFetcher{client: client, identifier: "orders-db"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected a real (non-not-found) API error to propagate, got nil")
	}
}
