package eks

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
)

// Discover fetches every EKS cluster name (unfiltered), for `ctl discover
// aws`. ListClusters returns only names; use `ctl aws eks def <cluster>` for
// full configuration.
func Discover(ctx context.Context, client eksAPI) ([]snapshot.Resource, error) {
	var resources []snapshot.Resource
	paginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, name := range page.Clusters {
			resources = append(resources, snapshot.Resource{ID: name, Type: "eks:cluster", Attrs: name})
		}
	}
	return resources, nil
}
