package s3

import (
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/viewer"
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// listBucketsAPI is the minimal client capability listBucket needs, letting
// tests substitute a fake instead of a real *s3.Client.
type listBucketsAPI interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

func listBucket(ctx context.Context, client listBucketsAPI, timeout globals.RequestTimeout) (*s3.ListBucketsOutput, *aws.ErrorInfo) {
	connectTimeout := time.Duration(timeout.ConnectionTimeout) * time.Second
	if connectTimeout <= 0 {
		// Guard against an already-expired context if a caller ever forgets to
		// populate timeout (as NewBucketListCommandExecutor used to).
		connectTimeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	apiOutput, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		errorInfo := aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil)
		return nil, errorInfo
	}
	if len(apiOutput.Buckets) == 0 {
		errorInfo := &aws.ErrorInfo{Err: NoBucketFound(), ErrorType: viewer.INFO}
		return nil, errorInfo
	}
	return apiOutput, nil
}

// listObjectsAPI is the minimal client capability fetchBucketObjects needs,
// letting tests substitute a fake instead of a real *s3.Client.
type listObjectsAPI interface {
	ListObjects(ctx context.Context, params *s3.ListObjectsInput, optFns ...func(*s3.Options)) (*s3.ListObjectsOutput, error)
}

// bucketConfigurationAPI is the minimal client capability
// bucketConfigurationFetcher needs, letting tests substitute a fake instead
// of a real *s3.Client.
type bucketConfigurationAPI interface {
	GetBucketPolicy(ctx context.Context, params *s3.GetBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error)
	GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
	GetBucketTagging(ctx context.Context, params *s3.GetBucketTaggingInput, optFns ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error)
	GetBucketEncryption(ctx context.Context, params *s3.GetBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error)
	GetBucketLifecycleConfiguration(ctx context.Context, params *s3.GetBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error)
}

// fetchBucketObjects paginates through a bucket's objects up to maxKeys.
// notice carries a non-fatal INFO note (the bucket has more objects than
// maxKeys) that should be shown alongside otherwise-valid results; err is
// only set for a genuine fetch failure or an empty result.
func fetchBucketObjects(ctx context.Context, bucketName string, objectPrefix *string, maxKeys int32, client listObjectsAPI) (objects []types.Object, notice *aws.ErrorInfo, err error) {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()

	var fetch func(remainingKeys int32, objectsPtr *[]types.Object, marker *string) error
	fetch = func(remainingKeys int32, objectsPtr *[]types.Object, marker *string) error {
		if remainingKeys == 0 { // terminate condition
			if marker != nil {
				notice = aws.NewErrorInfo(BucketContainMoreObject(bucketName, maxKeys), viewer.INFO, nil)
			}
			return nil
		}

		input := &s3.ListObjectsInput{
			Bucket:  &bucketName,
			Prefix:  objectPrefix,
			MaxKeys: &remainingKeys,
			Marker:  marker,
		}
		apiOutput, apiErr := client.ListObjects(ctx, input)
		if apiErr != nil {
			return aws.NewErrorInfo(aws.AWSError(apiErr), viewer.ERROR, nil)
		}
		apiOutputLen := len(apiOutput.Contents)
		*objectsPtr = append(*objectsPtr, apiOutput.Contents...)
		if apiOutput.IsTruncated != nil && *apiOutput.IsTruncated && apiOutputLen > 0 {
			nextMarker := apiOutput.Contents[apiOutputLen-1].Key
			return fetch(remainingKeys-int32(apiOutputLen), objectsPtr, nextMarker)
		}
		return nil
	}

	result := []types.Object{}
	nextMarker := "" // empty marker to start process
	if fetchErr := fetch(maxKeys, &result, &nextMarker); fetchErr != nil {
		return nil, nil, fetchErr
	}
	if len(result) == 0 {
		return nil, nil, aws.NewErrorInfo(NoObjectFound(bucketName), viewer.INFO, nil)
	}
	return result, notice, nil
}
