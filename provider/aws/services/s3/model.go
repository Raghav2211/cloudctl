package s3

import (
	"cloudctl/provider/aws"
	ctltime "cloudctl/time"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type bucketOjectsDownloadSummary struct {
	bucketName             string
	objectsDownloadSummary []*objectDownloadSummary
	err                    *aws.ErrorInfo
}

type objectDownloadSummary struct {
	source      string
	destination string
	sizeinBytes int64
	timeElapsed time.Duration
	err         *aws.ErrorInfo
}

type bucketOutput struct {
	name         *string
	creationDate *time.Time
}
type bucketObjectOutput struct {
	key          *string
	sizeInBytes  *int64
	storageClass *string
	lastModified *time.Time
}
type bucketListOutput struct {
	buckets []*bucketOutput
	err     *aws.ErrorInfo
}

type bucketObjectListOutput struct {
	bucketName *string
	objects    []*bucketObjectOutput
	err        *aws.ErrorInfo
}

type bucketDefinition struct {
	bucketName               *string
	policy                   *interface{}
	policyAPIErr             error
	version                  *interface{}
	versionAPIErr            error
	tags                     *interface{}
	tagsAPIError             error
	encryptionConfig         *interface{}
	encryptionConfigAPIError error
	lifecycle                *interface{}
	lifeCycleAPIError        error
}

func newBucketOutput(bucket *s3.Bucket, tz *ctltime.Timezone) *bucketOutput {
	return &bucketOutput{
		name:         bucket.Name,
		creationDate: tz.AdaptTimezone(bucket.CreationDate),
	}
}

func newBucketObjectOutput(o *s3.Object, tz *ctltime.Timezone) *bucketObjectOutput {
	return &bucketObjectOutput{
		key:          o.Key,
		sizeInBytes:  o.Size,
		storageClass: o.StorageClass,
		lastModified: tz.AdaptTimezone(o.LastModified),
	}
}

func newBucketObjectDownloadSummary(key, fileName string, numBytesWrite int64, timeElapsed time.Duration, err *aws.ErrorInfo) *objectDownloadSummary {
	return &objectDownloadSummary{
		source:      key,
		destination: fileName,
		sizeinBytes: numBytesWrite,
		timeElapsed: timeElapsed,
		err:         err,
	}
}

func (o *bucketDefinition) SetBucketName(bucketName string) *bucketDefinition {
	o.bucketName = &bucketName
	return o
}

func (o *bucketDefinition) SetPolicy(data interface{}) *bucketDefinition {
	o.policy = &data
	return o
}

func (o *bucketDefinition) SetVersion(data interface{}) *bucketDefinition {
	o.version = &data
	return o
}

func (o *bucketDefinition) SetTags(data interface{}) *bucketDefinition {
	o.tags = &data
	return o
}

func (o *bucketDefinition) SetEncryptionConfig(data interface{}) *bucketDefinition {
	o.encryptionConfig = &data
	return o
}
func (o *bucketDefinition) SetLifeCycle(data interface{}) *bucketDefinition {
	o.lifecycle = &data
	return o
}

func (o *bucketDefinition) SetPolicyAPIError(err error) *bucketDefinition {
	o.policyAPIErr = err
	return o
}

func (o *bucketDefinition) SetVersionAPIError(err error) *bucketDefinition {
	o.versionAPIErr = err
	return o
}

func (o *bucketDefinition) SetTagsAPIError(err error) *bucketDefinition {
	o.tagsAPIError = err
	return o
}

func (o *bucketDefinition) SetEncryptionConfigAPIError(err error) *bucketDefinition {
	o.encryptionConfigAPIError = err
	return o
}
func (o *bucketDefinition) SetLifeCycleError(err error) *bucketDefinition {
	o.lifeCycleAPIError = err
	return o
}

func (o bucketDefinition) Pretty() {
	if o.encryptionConfigAPIError != nil {
		fmt.Println("encryptionConfigAPIError", o.encryptionConfigAPIError)
	} else {
		fmt.Println("encryptionConfig", *o.encryptionConfig)
	}
	if o.tagsAPIError != nil {
		fmt.Println("tagsAPIError", o.tagsAPIError)
	} else {
		fmt.Println("tags", *o.tags)
	}
	if o.policyAPIErr != nil {
		fmt.Println("policyAPIErr", o.policyAPIErr)
	} else {
		fmt.Println("policy", *o.policy)
	}
	if o.versionAPIErr != nil {
		fmt.Println("versionAPIErr", o.versionAPIErr)
	} else {
		fmt.Println("versioning", *o.version)
	}
	if o.lifeCycleAPIError != nil {

		fmt.Println("lifeCycleAPIError", o.lifeCycleAPIError)
	} else {
		fmt.Println("lifecycle", *o.lifecycle)
	}

}
