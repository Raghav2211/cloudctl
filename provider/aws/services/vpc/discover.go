package vpc

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Discover fetches every VPC's full attributes (unfiltered), for `ctl
// discover aws`. DescribeVpcs already returns full detail per VPC in the
// list call itself, so — unlike dynamodb/eks's name-only discovery — no
// extra describe call is needed to preserve full raw SDK detail (§10).
func Discover(ctx context.Context, client vpcAPI) ([]snapshot.Resource, error) {
	var resources []snapshot.Resource
	paginator := ec2.NewDescribeVpcsPaginator(client, &ec2.DescribeVpcsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Vpcs {
			resources = append(resources, snapshot.Resource{ID: derefStr(v.VpcId), Type: "vpc:vpc", Attrs: v})
		}
	}
	return resources, nil
}
