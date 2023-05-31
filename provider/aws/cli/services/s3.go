package services

import (
	"cloudctl/provider/aws/cli/globals"
	ctls3 "cloudctl/provider/aws/services/s3"
)

type listCmd struct {
	CreationDateFrom string `name:"from" help:"Get list of bucket which start from this date(inclusive)"`
	CreationDateTO   string `name:"to" help:"Get list of bucket which start to this date(inclusive)"`
}

type listBucketObjectsCmd struct {
	BucketPrefix string `name:"prefix"`
	BucketName   string `name:"name" arg:"required"`
}

type bucketViewCmd struct {
	BucketName string `name:"name" arg:"required"`
}

type bucketObjectDownloadCmd struct {
	BucketName string `name:"name" arg:"required"`
	Key        string `name:"key" arg:"required"`
	Path       string `name:"path" type:"path" help:"Path to store the object(s), Default is current directory" arg:"required" default:"."`
	Recursive  bool   `name:"recursive" help:"Download objects recursively from provided key as prefix"`
}

type S3Command struct {
	List                 listCmd                 `name:"ls" cmd:"" help:"List s3 buckets"`
	ListBucketObjects    listBucketObjectsCmd    `name:"list-objects" cmd:"" help:"Bucket Objects list"`
	BucketView           bucketViewCmd           `name:"config" cmd:"" help:"Bucket configuration"`
	BucketObjectDownload bucketObjectDownloadCmd `name:"get" cmd:"" help:"Download bucket object(s)"`
}

func (cmd *listCmd) Run(flag *globals.CLIFlag) error {
	icmd := ctls3.NewBucketListCommandExecutor(flag, cmd.CreationDateFrom, cmd.CreationDateTO)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *listBucketObjectsCmd) Run(flag *globals.CLIFlag) error {
	icmd := ctls3.NewBucketObjectListCommandExecutor(flag, cmd.BucketName, cmd.BucketPrefix)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func (cmd *bucketViewCmd) Run(flag *globals.CLIFlag) error {
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
