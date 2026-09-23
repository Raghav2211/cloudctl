package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/s3"
	"context"
)

type listCmd struct {
	globals.AWSCLIFlag
	BucketNameInString   *string `name:"name" help:"List of bucket which contains provided value in their name"`
	CreationDateInString *string `name:"createAt" help:"The time when the bucket was created, in the ISO 8601 format in the UTC time zone (YYYY-MM-DDThh:mm:ss.sssZ), for example, 2021-09-29T11:04:43.305Z. You can use a wildcard (*), for example, 2021-09-29T*"`
}

type listBucketObjectsCmd struct {
	globals.AWSCLIFlag
	ObjectPrefix  *string `name:"prefix" help:"Bucket Object prefix"`
	MaxKeysReturn int32   `name:"max-keys" default:"1000" help:"Number of bucket objects return | Default value is 1000"`
	BucketName    string  `name:"name" arg:"required" help:"Bucket name"`
}

type bucketDefinitionCmd struct {
	globals.AWSCLIFlag
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
}

type bucketImpactCmd struct {
	globals.AWSCLIFlag
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
}

type bucketObjectDownloadCmd struct {
	globals.AWSCLIFlag
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
	Key        string `name:"key" arg:"required" help:"Bucket key or key prefix"`
	Path       string `name:"path" type:"path" help:"Path to local store the object(s), Default is current directory" arg:"required" default:"."`
	Recursive  bool   `name:"recursive" help:"This mode will download all objects recursively with provided key as prefix"`
}

type S3Command struct {
	List                 listCmd                 `name:"ls" cmd:"" help:"Return list s3 buckets"`
	ListBucketObjects    listBucketObjectsCmd    `name:"list-objects" cmd:"" help:"Return list of objects of s3 bucket"`
	BucketDefinition     bucketDefinitionCmd     `name:"def" cmd:"" help:"Return bucket definition"`
	BucketImpact         bucketImpactCmd         `name:"impact" cmd:"" help:"Cross-reference IAM policies against this bucket and narrate the blast radius"`
	BucketObjectDownload bucketObjectDownloadCmd `name:"get" cmd:"" help:"Download bucket object(s)"`
}

func (cmd *listCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	filter := s3.NewBucketListFilter(
		s3.WithBucketNameFilter(cmd.BucketNameInString),
		s3.WithCreationDateFilter(cmd.CreationDateInString),
	)

	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := s3.NewBucketListCommandExecutor(*cli, *session, cmd.AWSCLIFlag.RequestTimeout, filter)
	return icmd.Execute(ctx)
}

func (cmd *listBucketObjectsCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := s3.NewBucketObjectListCommandExecutor(*session, cmd.BucketName, cmd.ObjectPrefix, cmd.MaxKeysReturn)
	return icmd.Execute(ctx)
}

func (cmd *bucketDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := s3.NewBucketViewCommandExecutor(*session, cmd.BucketName)
	return icmd.Execute(ctx)
}

func (cmd *bucketImpactCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := s3.NewBucketImpactCommandExecutor(*session, cmd.BucketName)
	return icmd.Execute(ctx)
}

func (cmd *bucketObjectDownloadCmd) Run(ctx context.Context, cli *global.CLIFlag) error {

	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}

	icmd := s3.NewBucketObjectDownloadCommandExecutor(*session, cmd.BucketName, cmd.Key, cmd.Path, cmd.Recursive)
	return icmd.Execute(ctx)
}
