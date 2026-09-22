package s3

import (
	"cloudctl/provider/aws"
	ctltime "cloudctl/time"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type bucketOjectsDownloadSummary struct {
	bucketName             string
	objectsDownloadSummary []*objectDownloadSummary
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
}

// notice carries a non-fatal INFO note (e.g. the bucket has more objects than
// the requested max-keys) that should render alongside otherwise-valid objects.
type bucketObjectListOutput struct {
	bucketName *string
	objects    []*bucketObjectOutput
	notice     *aws.ErrorInfo
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

	// aiSummary is a Hypothesis-grade prose summary generated from the
	// Fact-tagged evidence gathered above (see fetcher.go). It is additive,
	// never a replacement for the raw data below — if empty,
	// aiSummaryUnavailable explains why (AI is never a hard dependency).
	aiSummary            string
	aiSummaryUnavailable string
}

func newBucketOutput(bucket types.Bucket, tz *ctltime.Timezone) *bucketOutput {
	return &bucketOutput{
		name:         bucket.Name,
		creationDate: tz.AdaptTimezone(bucket.CreationDate),
	}
}

func newBucketObjectOutput(o types.Object, tz *ctltime.Timezone) *bucketObjectOutput {
	return &bucketObjectOutput{
		key:          o.Key,
		sizeInBytes:  o.Size,
		storageClass: (*string)(&o.StorageClass.Values()[0]), // TODO : handle array
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

func (o *bucketDefinition) SetAISummary(summary string) *bucketDefinition {
	o.aiSummary = summary
	return o
}

func (o *bucketDefinition) SetAISummaryUnavailable(reason string) *bucketDefinition {
	o.aiSummaryUnavailable = reason
	return o
}

func (o bucketDefinition) Pretty() {
	fmt.Println("=== AI Summary (Hypothesis — verify against the raw data below) ===")
	if o.aiSummary != "" {
		fmt.Println(o.aiSummary)
	} else {
		reason := o.aiSummaryUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		fmt.Printf("summary unavailable: %s\n", reason)
	}
	fmt.Println()
	fmt.Println("=== Raw Data ===")

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
