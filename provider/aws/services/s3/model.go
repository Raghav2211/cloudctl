package s3

import (
	"cloudctl/provider/aws"
	ctltime "cloudctl/time"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	policy                   *s3.GetBucketPolicyOutput
	policyAPIErr             error
	version                  *s3.GetBucketVersioningOutput
	versionAPIErr            error
	tags                     *s3.GetBucketTaggingOutput
	tagsAPIError             error
	encryptionConfig         *s3.GetBucketEncryptionOutput
	encryptionConfigAPIError error
	lifecycle                *s3.GetBucketLifecycleConfigurationOutput
	lifeCycleAPIError        error

	// aiSummary is a Hypothesis-grade prose summary generated from the
	// Fact-tagged evidence gathered above (see fetcher.go). It is additive,
	// never a replacement for the raw data below — if empty,
	// aiSummaryUnavailable explains why (AI is never a hard dependency).
	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
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
		storageClass: (*string)(&o.StorageClass),
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

func (o *bucketDefinition) SetPolicy(data *s3.GetBucketPolicyOutput) *bucketDefinition {
	o.policy = data
	return o
}

func (o *bucketDefinition) SetVersion(data *s3.GetBucketVersioningOutput) *bucketDefinition {
	o.version = data
	return o
}

func (o *bucketDefinition) SetTags(data *s3.GetBucketTaggingOutput) *bucketDefinition {
	o.tags = data
	return o
}

func (o *bucketDefinition) SetEncryptionConfig(data *s3.GetBucketEncryptionOutput) *bucketDefinition {
	o.encryptionConfig = data
	return o
}
func (o *bucketDefinition) SetLifeCycle(data *s3.GetBucketLifecycleConfigurationOutput) *bucketDefinition {
	o.lifecycle = data
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

func (o *bucketDefinition) SetAIRecommendations(recommendations string) *bucketDefinition {
	o.aiRecommendations = recommendations
	return o
}

func (o *bucketDefinition) SetAIRecommendationsUnavailable(reason string) *bucketDefinition {
	o.aiRecommendationsUnavailable = reason
	return o
}
