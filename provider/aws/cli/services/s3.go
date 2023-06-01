package services

import (
	"cloudctl/provider/aws/cli/globals"
	ctls3 "cloudctl/provider/aws/services/s3"
)

type listCmd struct {
	BucketNameString string `name:"name" help:"Get list of bucket which contains provided value in their name"`
	CreationDateFrom string `name:"from" help:"Get list of bucket which starts from provided date(inclusive)"`
	CreationDateTO   string `name:"to" help:"Get list of bucket which ends to provided date(inclusive)"`
}

type listBucketObjectsCmd struct {
	ObjectPrefix  string `name:"prefix" help:"Bucket Object prefix"`
	MaxKeysReturn int    `name:"max-keys" default:"1000" help:"Number of bucket objects return | Default value is 1000"`
	Full          bool   `name:"all" help:"It's a heavy operation & will take cost. This mode will list all bucket objects with applied filter"`
	BucketName    string `name:"name" arg:"required" help:"Bucket name"`
}

type bucketDefinitionCmd struct {
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
}

type bucketObjectDownloadCmd struct {
	BucketName string `name:"name" arg:"required" help:"Bucket name"`
	Key        string `name:"key" arg:"required" help:"Bucket Key or Key prefix"`
	Path       string `name:"path" type:"path" help:"Path to local store the object(s), Default is current directory" arg:"required" default:"."`
	Recursive  bool   `name:"recursive" help:"This mode will download all objects recursively with provided key as prefix"`
}

type S3Command struct {
	List                 listCmd                 `name:"ls" cmd:"" help:"List s3 buckets"`
	ListBucketObjects    listBucketObjectsCmd    `name:"list-objects" cmd:"" help:"List s3 bucket objects"`
	BucketDefinition     bucketDefinitionCmd     `name:"def" cmd:"" help:"Get bucket definition"`
	BucketObjectDownload bucketObjectDownloadCmd `name:"get" cmd:"" help:"Download bucket object(s)"`
}

func (cmd *listCmd) Run(flag *globals.CLIFlag) error {
	icmd := ctls3.NewBucketListCommandExecutor(flag, cmd.CreationDateFrom, cmd.CreationDateTO, cmd.BucketNameString)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *listBucketObjectsCmd) Run(flag *globals.CLIFlag) error {
	icmd := ctls3.NewBucketObjectListCommandExecutor(flag, cmd.BucketName, cmd.ObjectPrefix)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *bucketDefinitionCmd) Run(flag *globals.CLIFlag) error {
	icmd := ctls3.NewBucketViewCommandExecutor(flag, cmd.BucketName)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *bucketObjectDownloadCmd) Run(flag *globals.CLIFlag) error {
	icmd := ctls3.NewBucketObjectDownloadCommandExecutor(flag, cmd.BucketName, cmd.Key, cmd.Path, cmd.Recursive)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}
