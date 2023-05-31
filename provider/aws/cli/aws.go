package cli

import "cloudctl/provider/aws/cli/services"

type AWSCmd struct {
	S3  services.S3Command  `name:"s3" cmd:"" help:"Operation on S3 buckets"`
	EC2 services.EC2Command `name:"ec2" cmd:"" help:"Operation on ec2"`
}
