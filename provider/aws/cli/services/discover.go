package services

import (
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/dynamodb"
	"cloudctl/provider/aws/services/ec2"
	"cloudctl/provider/aws/services/eks"
	"cloudctl/provider/aws/services/lambda"
	"cloudctl/provider/aws/services/rds"
	"cloudctl/provider/aws/services/s3"
	"cloudctl/provider/aws/services/vpc"
	"cloudctl/snapshot"
	"context"
	"fmt"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
)

type DiscoverAWSCmd struct {
	globals.AWSCLIFlag
}

// Run discovers EC2 instances and S3 buckets and persists them into a local
// snapshot, so later commands can query `--from-snapshot` instead of always
// hitting live APIs (§9 Stage 1 / ADR-008). Builds one session for both
// providers (ADR-013), rather than each Discover function resolving
// credentials separately.
func (cmd DiscoverAWSCmd) Run(ctx context.Context) error {
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		return err
	}
	defer store.Close()

	snapshotID, err := store.NewSnapshot(ctx, "aws")
	if err != nil {
		return err
	}

	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(cmd.AWSCLIFlag, false))
	if err != nil {
		return err
	}

	ec2Client := awsec2.NewFromConfig(*cfg)

	ec2Resources, err := ec2.Discover(ctx, ec2Client)
	if err != nil {
		return fmt.Errorf("discovering EC2 instances: %w", err)
	}

	s3Resources, err := s3.Discover(ctx, *cfg)
	if err != nil {
		return fmt.Errorf("discovering S3 buckets: %w", err)
	}

	dynamodbResources, err := dynamodb.Discover(ctx, awsdynamodb.NewFromConfig(*cfg))
	if err != nil {
		return fmt.Errorf("discovering DynamoDB tables: %w", err)
	}

	eksResources, err := eks.Discover(ctx, awseks.NewFromConfig(*cfg))
	if err != nil {
		return fmt.Errorf("discovering EKS clusters: %w", err)
	}

	rdsResources, err := rds.Discover(ctx, awsrds.NewFromConfig(*cfg))
	if err != nil {
		return fmt.Errorf("discovering RDS instances/clusters: %w", err)
	}

	vpcResources, err := vpc.Discover(ctx, ec2Client)
	if err != nil {
		return fmt.Errorf("discovering VPCs: %w", err)
	}

	lambdaResources, err := lambda.Discover(ctx, awslambda.NewFromConfig(*cfg))
	if err != nil {
		return fmt.Errorf("discovering Lambda functions: %w", err)
	}

	all := append(ec2Resources, s3Resources...)
	all = append(all, dynamodbResources...)
	all = append(all, eksResources...)
	all = append(all, rdsResources...)
	all = append(all, vpcResources...)
	all = append(all, lambdaResources...)

	for _, r := range all {
		if err := store.SaveResource(ctx, snapshotID, "aws", r); err != nil {
			return err
		}
	}

	fmt.Printf("Discovered %d EC2 instance(s), %d S3 bucket(s), %d DynamoDB table(s), %d EKS cluster(s), %d RDS instance/cluster(s), %d VPC(s), and %d Lambda function(s) into snapshot %d (%s)\n",
		len(ec2Resources), len(s3Resources), len(dynamodbResources), len(eksResources), len(rdsResources), len(vpcResources), len(lambdaResources), snapshotID, snapshot.DefaultPath())
	return nil
}
