package ec2

import (
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Discover fetches every EC2 instance in the account/region (unfiltered),
// for `ctl discover aws`. Each instance's full raw SDK representation is
// preserved in Attrs (§10: attrs_json preserves provider-specific detail
// rather than a hand-picked field subset).
func Discover(ctx context.Context, flag *globals.AWSCLIFlag) ([]snapshot.Resource, error) {
	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(*flag, true))
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(*cfg)

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
