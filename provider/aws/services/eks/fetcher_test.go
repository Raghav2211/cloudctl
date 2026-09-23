package eks

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// fakeEKSClient implements eksAPI with canned responses, letting tests
// substitute it for a real *eks.Client (ADR-007).
type fakeEKSClient struct {
	listOut *eks.ListClustersOutput
	listErr error

	describeOut *eks.DescribeClusterOutput
	describeErr error

	listNGOut *eks.ListNodegroupsOutput
	listNGErr error

	describeNGOut map[string]*eks.DescribeNodegroupOutput
	describeNGErr error
}

func (f *fakeEKSClient) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listOut == nil {
		return &eks.ListClustersOutput{}, nil
	}
	return f.listOut, nil
}

func (f *fakeEKSClient) DescribeCluster(_ context.Context, _ *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	return f.describeOut, nil
}

func (f *fakeEKSClient) ListNodegroups(_ context.Context, _ *eks.ListNodegroupsInput, _ ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	if f.listNGErr != nil {
		return nil, f.listNGErr
	}
	if f.listNGOut == nil {
		return &eks.ListNodegroupsOutput{}, nil
	}
	return f.listNGOut, nil
}

func (f *fakeEKSClient) DescribeNodegroup(_ context.Context, params *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	if f.describeNGErr != nil {
		return nil, f.describeNGErr
	}
	out, ok := f.describeNGOut[*params.NodegroupName]
	if !ok {
		return nil, errors.New("nodegroup not found")
	}
	return out, nil
}

func testCluster(name string) types.Cluster {
	return types.Cluster{
		Name:     aws.String(name),
		Version:  aws.String("1.31"),
		Status:   types.ClusterStatusActive,
		Endpoint: aws.String("https://" + name + ".eks.eu-west-1.amazonaws.com"),
		ResourcesVpcConfig: &types.VpcConfigResponse{
			EndpointPublicAccess:  false,
			EndpointPrivateAccess: true,
		},
		Logging: &types.Logging{
			ClusterLogging: []types.LogSetup{
				{Enabled: aws.Bool(true), Types: []types.LogType{types.LogTypeApi, types.LogTypeAudit}},
				{Enabled: aws.Bool(false), Types: []types.LogType{types.LogTypeScheduler}},
			},
		},
	}
}

func TestClusterListFetcher_Fetch_HappyPath(t *testing.T) {
	client := &fakeEKSClient{listOut: &eks.ListClustersOutput{Clusters: []string{"prod", "qa"}}}
	f := clusterListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(out.clusters))
	}
}

func TestClusterListFetcher_Fetch_EmptyResult(t *testing.T) {
	f := clusterListFetcher{client: &fakeEKSClient{}}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no clusters are found, got nil")
	}
}

func TestClusterListFetcher_Fetch_APIError(t *testing.T) {
	f := clusterListFetcher{client: &fakeEKSClient{listErr: errors.New("boom")}}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing ListClusters call, got nil")
	}
}

func TestClusterDefinitionFetcher_Fetch_EndToEnd(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	ng := types.Nodegroup{
		NodegroupName: aws.String("default"),
		Status:        types.NodegroupStatusActive,
		CapacityType:  types.CapacityTypesOnDemand,
		InstanceTypes: []string{"m5.large"},
		ScalingConfig: &types.NodegroupScalingConfig{DesiredSize: aws.Int32(3), MinSize: aws.Int32(1), MaxSize: aws.Int32(5)},
	}
	client := &fakeEKSClient{
		describeOut: &eks.DescribeClusterOutput{Cluster: ptr(testCluster("prod"))},
		listNGOut:   &eks.ListNodegroupsOutput{Nodegroups: []string{"default"}},
		describeNGOut: map[string]*eks.DescribeNodegroupOutput{
			"default": {Nodegroup: &ng},
		},
	}
	f := clusterDefinitionFetcher{client: client, clusterName: "prod"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if derefStr(def.name) != "prod" {
		t.Errorf("expected name 'prod', got %q", derefStr(def.name))
	}
	if len(def.nodeGroups) != 1 {
		t.Fatalf("expected 1 node group, got %d", len(def.nodeGroups))
	}
	if !def.endpointPrivateAccess || def.endpointPublicAccess {
		t.Error("expected private-only endpoint access")
	}
	if len(def.enabledClusterLogTypes) != 2 {
		t.Errorf("expected 2 enabled log types (api, audit), got %d: %v", len(def.enabledClusterLogTypes), def.enabledClusterLogTypes)
	}

	clusterDefinitionViewer(def, nil).View() // must not panic
}

// TestClusterDefinitionFetcher_Fetch_SkipsFailedNodeGroup confirms a single
// node group whose DescribeNodegroup call fails is skipped rather than
// failing the whole command — mirroring the partial-failure tolerance
// established for s3's bucketConfigurationFetcher and rds's PITR/TTL
// sub-fetches.
func TestClusterDefinitionFetcher_Fetch_SkipsFailedNodeGroup(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	client := &fakeEKSClient{
		describeOut:   &eks.DescribeClusterOutput{Cluster: ptr(testCluster("prod"))},
		listNGOut:     &eks.ListNodegroupsOutput{Nodegroups: []string{"broken-group"}},
		describeNGOut: map[string]*eks.DescribeNodegroupOutput{}, // "broken-group" intentionally absent -> DescribeNodegroup errors
	}
	f := clusterDefinitionFetcher{client: client, clusterName: "prod"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected a failed node-group describe not to fail the whole fetch: %v", err)
	}
	if len(def.nodeGroups) != 0 {
		t.Errorf("expected the failed node group to be skipped, got %d node groups", len(def.nodeGroups))
	}

	clusterDefinitionViewer(def, nil).View() // must not panic
}

func TestClusterDefinitionFetcher_Fetch_NotFound(t *testing.T) {
	f := clusterDefinitionFetcher{client: &fakeEKSClient{describeOut: &eks.DescribeClusterOutput{}}, clusterName: "does-not-exist"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when the cluster doesn't exist, got nil")
	}
}

func TestClusterDefinitionFetcher_Fetch_APIError(t *testing.T) {
	f := clusterDefinitionFetcher{client: &fakeEKSClient{describeErr: errors.New("boom")}, clusterName: "prod"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeCluster call, got nil")
	}
}

func ptr[T any](v T) *T { return &v }
