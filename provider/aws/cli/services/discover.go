package services

import (
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/ec2"
	"cloudctl/provider/aws/services/s3"
	"cloudctl/snapshot"
	"context"
	"fmt"
)

type DiscoverAWSCmd struct {
	globals.AWSCLIFlag
}

// Run discovers EC2 instances and S3 buckets and persists them into a local
// snapshot, so later commands can query `--from-snapshot` instead of always
// hitting live APIs (§9 Stage 1 / ADR-008).
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

	ec2Resources, err := ec2.Discover(ctx, &cmd.AWSCLIFlag)
	if err != nil {
		return fmt.Errorf("discovering EC2 instances: %w", err)
	}

	cfg, err := aws.NewSessionV2(aws.NewCredentialConfig(cmd.AWSCLIFlag, false))
	if err != nil {
		return err
	}
	s3Resources, err := s3.Discover(ctx, *cfg)
	if err != nil {
		return fmt.Errorf("discovering S3 buckets: %w", err)
	}

	for _, r := range append(ec2Resources, s3Resources...) {
		if err := store.SaveResource(ctx, snapshotID, "aws", r); err != nil {
			return err
		}
	}

	fmt.Printf("Discovered %d EC2 instance(s) and %d S3 bucket(s) into snapshot %d (%s)\n",
		len(ec2Resources), len(s3Resources), snapshotID, snapshot.DefaultPath())
	return nil
}
