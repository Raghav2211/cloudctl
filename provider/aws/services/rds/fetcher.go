package rds

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// dbAPI is the minimal client capability this package needs, letting tests
// substitute a fake instead of a real *rds.Client (ADR-007).
type dbAPI interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

type dbListFetcher struct {
	client dbAPI
}

type dbDefinitionFetcher struct {
	client     dbAPI
	identifier string
}

// isNotFound reports whether err is RDS's typed "no such instance/cluster"
// fault — used to fall through from instance lookup to cluster lookup
// (Aurora clusters and standalone instances share one `def <identifier>`
// command but live in separate identifier namespaces) without treating a
// simple "wrong namespace" miss as a real API failure.
func isNotFound(err error) bool {
	var instNotFound *types.DBInstanceNotFoundFault
	var clusterNotFound *types.DBClusterNotFoundFault
	return errors.As(err, &instNotFound) || errors.As(err, &clusterNotFound)
}

// Fetch lists every DB instance and DB cluster in the account — RDS exposes
// standalone instances and Aurora clusters as two separate paginated
// resource types, so both are fetched and combined into one list.
func (f dbListFetcher) Fetch(ctx context.Context) (*dbListOutput, error) {
	var items []*dbSummary

	instPaginator := rds.NewDescribeDBInstancesPaginator(f.client, &rds.DescribeDBInstancesInput{})
	for instPaginator.HasMorePages() {
		page, err := instPaginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, inst := range page.DBInstances {
			items = append(items, newDBSummaryFromInstance(inst))
		}
	}

	clusterPaginator := rds.NewDescribeDBClustersPaginator(f.client, &rds.DescribeDBClustersInput{})
	for clusterPaginator.HasMorePages() {
		page, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, c := range page.DBClusters {
			items = append(items, newDBSummaryFromCluster(c))
		}
	}

	if len(items) == 0 {
		return nil, ctlaws.NewErrorInfo(NoDatabaseFound(), viewer.INFO, nil)
	}
	return &dbListOutput{items: items}, nil
}

// Fetch tries the given identifier as a DB instance first, then as a DB
// cluster if that comes back not-found — the two share this one `def`
// command but are otherwise distinct resource types with independent
// identifier namespaces.
func (f dbDefinitionFetcher) Fetch(ctx context.Context) (*dbDefinition, error) {
	instOut, err := f.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{DBInstanceIdentifier: &f.identifier})
	if err == nil && len(instOut.DBInstances) > 0 {
		def := newDBDefinitionFromInstance(instOut.DBInstances[0])
		def.applyAINarration(ctx, ai.NewClientFromEnv(), dbDefinitionEvidence(def))
		return def, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	clusterOut, err := f.client.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{DBClusterIdentifier: &f.identifier})
	if err == nil && len(clusterOut.DBClusters) > 0 {
		def := newDBDefinitionFromCluster(clusterOut.DBClusters[0])
		def.applyAINarration(ctx, ai.NewClientFromEnv(), dbDefinitionEvidence(def))
		return def, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	return nil, ctlaws.NewErrorInfo(DatabaseNotFound(f.identifier), viewer.INFO, nil)
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring dynamodb's applyAINarration.
func (def *dbDefinition) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
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
