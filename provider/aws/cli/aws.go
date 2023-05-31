package cli

type AWSCmd struct {
	S3  s3Command  `name:"s3" cmd:"" help:"Operation on S3 buckets"`
	EC2 eC2Command `name:"ec2" cmd:"" help:"Operation on ec2"`
}
