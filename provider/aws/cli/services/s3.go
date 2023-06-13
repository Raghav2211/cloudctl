package services

import (
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/s3"
)

type listCmd struct {
	BucketNameInString   *string `name:"name" help:"List of bucket which contains provided value in their name"`
	CreationDateInString *string `name:"createAt" help:"The time when the bucket was created, in the ISO 8601 format in the UTC time zone (YYYY-MM-DDThh:mm:ss.sssZ), for example, 2021-09-29T11:04:43.305Z. You can use a wildcard (*), for example, 2021-09-29T*"`
}

type listBucketObjectsCmd struct {
	ObjectPrefix  *string `name:"prefix" help:"Bucket Object prefix"`
	MaxKeysReturn int64   `name:"max-keys" default:"100" help:"Number of bucket objects return | Default value is 100"`
	BucketName    string  `name:"name" arg:"required" help:"Bucket name"`
}

type bucketDefinitionCmd struct {
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
}

type bucketObjectDownloadCmd struct {
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
	Key        string `name:"key" arg:"required" help:"Bucket key or key prefix"`
	Path       string `name:"path" type:"path" help:"Path to local store the object(s), Default is current directory" arg:"required" default:"."`
	Recursive  bool   `name:"recursive" help:"This mode will download all objects recursively with provided key as prefix"`
}

type S3Command struct {
	List                 listCmd                 `name:"ls" cmd:"" help:"Return list s3 buckets"`
	ListBucketObjects    listBucketObjectsCmd    `name:"list-objects" cmd:"" help:"Return list of objects of s3 bucket"`
	BucketDefinition     bucketDefinitionCmd     `name:"def" cmd:"" help:"Return bucket definition"`
	BucketObjectDownload bucketObjectDownloadCmd `name:"get" cmd:"" help:"Download bucket object(s)"`
}

func (cmd *listCmd) Run(flag *globals.CLIFlag) error {

	filter := s3.NewBucketListFilter(
		s3.WithBucketNameFilter(cmd.BucketNameInString),
		s3.WithCreationDateFilter(cmd.CreationDateInString),
	)

	icmd := s3.NewBucketListCommandExecutor(flag, filter)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *listBucketObjectsCmd) Run(flag *globals.CLIFlag) error {
	icmd := s3.NewBucketObjectListCommandExecutor(flag, cmd.BucketName, cmd.ObjectPrefix, cmd.MaxKeysReturn)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *bucketDefinitionCmd) Run(flag *globals.CLIFlag) error {
	icmd := s3.NewBucketViewCommandExecutor(flag, cmd.BucketName)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *bucketObjectDownloadCmd) Run(flag *globals.CLIFlag) error {
	icmd := s3.NewBucketObjectDownloadCommandExecutor(flag, cmd.BucketName, cmd.Key, cmd.Path, cmd.Recursive)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}
