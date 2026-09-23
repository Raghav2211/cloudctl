package cli

import (
	"cloudctl/provider/aws/cli/services"
)

type AWSCmd struct {
	S3       services.S3Command       `name:"s3" cmd:"" help:"Operation on S3 buckets"`
	EC2      services.EC2Command      `name:"ec2" cmd:"" help:"Operation on ec2"`
	DynamoDB services.DynamoDBCommand `name:"dynamodb" cmd:"" help:"Operation on DynamoDB tables"`
	RDS      services.RDSCommand      `name:"rds" cmd:"" help:"Operation on RDS instances and clusters"`
	VPC      services.VPCCommand      `name:"vpc" cmd:"" help:"Operation on VPCs"`
	EKS      services.EKSCommand      `name:"eks" cmd:"" help:"Operation on EKS clusters"`
}
