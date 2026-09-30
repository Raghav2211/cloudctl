package rds

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// Discover fetches every RDS DB instance and DB cluster's full attributes
// (unfiltered), for `ctl discover aws`. Both list calls already return full
// detail per item, so no extra describe call is needed (§10) — mirrors
// dbListFetcher.Fetch's pagination, minus its "zero results" business error,
// which isn't a discovery failure.
func Discover(ctx context.Context, client dbAPI) ([]snapshot.Resource, error) {
	var resources []snapshot.Resource

	instPaginator := rds.NewDescribeDBInstancesPaginator(client, &rds.DescribeDBInstancesInput{})
	for instPaginator.HasMorePages() {
		page, err := instPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, inst := range page.DBInstances {
			resources = append(resources, snapshot.Resource{ID: derefStr(inst.DBInstanceIdentifier), Type: "rds:instance", Attrs: inst})
		}
	}

	clusterPaginator := rds.NewDescribeDBClustersPaginator(client, &rds.DescribeDBClustersInput{})
	for clusterPaginator.HasMorePages() {
		page, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range page.DBClusters {
			resources = append(resources, snapshot.Resource{ID: derefStr(c.DBClusterIdentifier), Type: "rds:cluster", Attrs: c})
		}
	}

	return resources, nil
}
