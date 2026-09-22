package s3

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Discover fetches every S3 bucket (unfiltered), for `ctl discover aws`.
// Each bucket's full raw SDK representation is preserved in Attrs (§10).
func Discover(ctx context.Context, cfg aws.Config) ([]snapshot.Resource, error) {
	client := s3.NewFromConfig(cfg)
	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	resources := make([]snapshot.Resource, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		name := ""
		if b.Name != nil {
			name = *b.Name
		}
		resources = append(resources, snapshot.Resource{
			ID:    name,
			Type:  "s3:bucket",
			Attrs: b,
		})
	}
	return resources, nil
}
