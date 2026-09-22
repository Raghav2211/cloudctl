package cli

import "cloudctl/provider/aws/cli/services"

type DiscoverCmd struct {
	AWS services.DiscoverAWSCmd `name:"aws" cmd:"" help:"Discover AWS EC2 instances and S3 buckets into a local snapshot"`
}
