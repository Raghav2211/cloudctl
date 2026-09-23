package eks

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
)

// eksAPI is the minimal client capability this package needs, letting tests
// substitute a fake instead of a real *eks.Client (ADR-007).
type eksAPI interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
}

type clusterListFetcher struct {
	client eksAPI
}

type clusterDefinitionFetcher struct {
	client      eksAPI
	clusterName string
}

func (f clusterListFetcher) Fetch(ctx context.Context) (*clusterListOutput, error) {
	var names []string
	paginator := eks.NewListClustersPaginator(f.client, &eks.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		names = append(names, page.Clusters...)
	}
	if len(names) == 0 {
		return nil, ctlaws.NewErrorInfo(NoClusterFound(), viewer.INFO, nil)
	}

	clusters := make([]*clusterSummary, 0, len(names))
	for _, n := range names {
		clusters = append(clusters, newClusterSummary(n))
	}
	return &clusterListOutput{clusters: clusters}, nil
}

// Fetch retrieves a cluster's configuration and every node group's details,
// then narrates the whole thing via Summarize, mirroring every other
// *DefinitionFetcher.Fetch in this codebase. A node group whose individual
// DescribeNodegroup call fails is skipped rather than failing the whole
// command — the same partial-failure tolerance already established for
// s3's bucketConfigurationFetcher and rds's PITR/TTL sub-fetches.
func (f clusterDefinitionFetcher) Fetch(ctx context.Context) (*clusterDefinition, error) {
	clusterOut, err := f.client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &f.clusterName})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if clusterOut.Cluster == nil {
		return nil, ctlaws.NewErrorInfo(NoClusterFound(), viewer.INFO, nil)
	}
	def := newClusterDefinition(*clusterOut.Cluster)

	var nodeGroupNames []string
	ngPaginator := eks.NewListNodegroupsPaginator(f.client, &eks.ListNodegroupsInput{ClusterName: &f.clusterName})
	for ngPaginator.HasMorePages() {
		page, err := ngPaginator.NextPage(ctx)
		if err != nil {
			break // node-group listing is additive detail, not required for the cluster itself
		}
		nodeGroupNames = append(nodeGroupNames, page.Nodegroups...)
	}

	var nodeGroups []*nodeGroupSummary
	for _, ngName := range nodeGroupNames {
		ngName := ngName
		ngOut, err := f.client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{ClusterName: &f.clusterName, NodegroupName: &ngName})
		if err != nil || ngOut.Nodegroup == nil {
			continue
		}
		nodeGroups = append(nodeGroups, newNodeGroupSummary(*ngOut.Nodegroup))
	}
	def.SetNodeGroups(nodeGroups)

	def.applyAISummary(ctx, ai.NewClientFromEnv(), clusterDefinitionEvidence(def))

	return def, nil
}

// applyAISummary sets aiSummary or aiSummaryUnavailable from the given
// evidence, never returning an error (ADR-010).
func (def *clusterDefinition) applyAISummary(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
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
