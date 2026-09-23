package ec2

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Discover fetches every EC2 instance in the account/region (unfiltered),
// for `ctl discover aws`. Each instance's full raw SDK representation is
// preserved in Attrs (§10: attrs_json preserves provider-specific detail
// rather than a hand-picked field subset).
//
// Takes an already-constructed client rather than building its own session
// (ADR-013) — `ctl discover aws` needs both EC2 and S3 data in one
// invocation, so the CLI layer builds one session and passes clients to
// both, instead of each Discover function resolving credentials separately.
func Discover(ctx context.Context, client ec2.DescribeInstancesAPIClient) ([]snapshot.Resource, error) {
	instances, err := fetchInstanceList(ctx, client, InstanceListFilter{})
	if err != nil {
		return nil, err
	}

	resources := make([]snapshot.Resource, 0, len(instances))
	for _, inst := range instances {
		resources = append(resources, snapshot.Resource{
			ID:    derefStr(inst.InstanceId),
			Type:  "ec2:instance",
			Attrs: inst,
		})
	}
	return resources, nil
}
