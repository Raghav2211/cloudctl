package eks

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"sync"

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

	def.applyAINarration(ctx, ai.NewClientFromEnv(), clusterDefinitionEvidence(def))

	return def, nil
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring dynamodb's applyAINarration.
func (def *clusterDefinition) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		def.SetAISummaryUnavailable("no evidence could be gathered")
		def.SetAIRecommendationsUnavailable("no evidence could be gathered")
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})

	if result.summaryErr != "" {
		def.SetAISummaryUnavailable(result.summaryErr)
	} else {
		def.SetAISummary(result.summary)
	}
	if result.recommendedErr != "" {
		def.SetAIRecommendationsUnavailable(result.recommendedErr)
	} else {
		def.SetAIRecommendations(result.recommendations)
	}
}
