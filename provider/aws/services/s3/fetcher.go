package s3

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	itime "cloudctl/time"
	"cloudctl/viewer"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"log"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type bucketListFetcher struct {
	client         listBucketsAPI
	requestTimeout globals.RequestTimeout
	filter         *BucketListFilter
	tz             *itime.Timezone
}

type bucketObjectsFetcher struct {
	client *s3.Client
	// fetch all objects for provided bucket
	bucketName   string
	objectPrefix *string
	maxKeys      int32
	tz           *itime.Timezone
}

type bucketConfigurationFetcher struct {
	client bucketConfigurationAPI
	// fetch configuration for provided bucket
	bucketName string
}

type bucketObjectsDownloadFetcher struct {
	client     *s3.Client
	downloader *manager.Downloader
	bucketName string
	key        string
	path       string
	recursive  bool
}

func (f bucketListFetcher) Fetch(ctx context.Context) (*bucketListOutput, error) {
	apiOutput, err := listBucket(ctx, f.client, f.requestTimeout)
	if err != nil {
		return nil, err
	}
	buckets := []*bucketOutput{}
	for _, o := range apiOutput.Buckets {
		if f.filter.applyCustomFilter(o) {
			buckets = append(buckets, newBucketOutput(o, f.tz))
		}
	}
	// default sort(asc) by creation date
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].creationDate.Before(*buckets[j].creationDate)
	})
	return &bucketListOutput{buckets: buckets}, nil
}

func (f bucketObjectsFetcher) Fetch(ctx context.Context) (*bucketObjectListOutput, error) {
	objects, notice, err := fetchBucketObjects(ctx, f.bucketName, f.objectPrefix, f.maxKeys, f.client)
	if err != nil {
		return nil, err
	}
	output := []*bucketObjectOutput{}
	for _, o := range objects {
		output = append(output, newBucketObjectOutput(o, f.tz))
	}
	return &bucketObjectListOutput{bucketName: &f.bucketName, objects: output, notice: notice}, nil
}

type policyResult struct {
	data *s3.GetBucketPolicyOutput
	err  error
}
type versionResult struct {
	data *s3.GetBucketVersioningOutput
	err  error
}
type tagsResult struct {
	data *s3.GetBucketTaggingOutput
	err  error
}
type encryptionResult struct {
	data *s3.GetBucketEncryptionOutput
	err  error
}
type lifecycleResult struct {
	data *s3.GetBucketLifecycleConfigurationOutput
	err  error
}

// Fetch gathers a bucket's policy/versioning/tags/encryption/lifecycle
// concurrently. Each dimension is fetched by its own goroutine into its own
// single-result channel (data and error together, not two separate
// channels) so there is exactly one value to receive per dimension — no
// ambiguity between "the real result arrived" and "the channel was closed
// with nothing in it" for select to race between.
func (f bucketConfigurationFetcher) Fetch(ctx context.Context) (*bucketDefinition, error) {
	definition := &bucketDefinition{}
	definition.SetBucketName(f.bucketName)

	policyCh := make(chan policyResult, 1)
	versionCh := make(chan versionResult, 1)
	tagsCh := make(chan tagsResult, 1)
	encryptionCh := make(chan encryptionResult, 1)
	lifecycleCh := make(chan lifecycleResult, 1)

	go func() {
		data, err := getBucketPolicy(ctx, &f.bucketName, f.client)
		policyCh <- policyResult{data: data, err: err}
	}()
	go func() {
		data, err := getBucketVersionConfig(ctx, &f.bucketName, f.client)
		versionCh <- versionResult{data: data, err: err}
	}()
	go func() {
		data, err := getBucketTags(ctx, &f.bucketName, f.client)
		tagsCh <- tagsResult{data: data, err: err}
	}()
	go func() {
		data, err := getBucketencryptionConfig(ctx, &f.bucketName, f.client)
		encryptionCh <- encryptionResult{data: data, err: err}
	}()
	go func() {
		data, err := getBucketLifecycleConfig(ctx, &f.bucketName, f.client)
		lifecycleCh <- lifecycleResult{data: data, err: err}
	}()

	// Local typed captures alongside the *interface{}-boxed fields set on
	// definition, so evidence can be built from concrete types below
	// without unboxing them again. Each channel is read exactly once, and
	// definition is only ever touched from this goroutine, so no mutex is
	// needed here.
	var policyData *s3.GetBucketPolicyOutput
	var versionData *s3.GetBucketVersioningOutput
	var tagsData *s3.GetBucketTaggingOutput
	var encryptionData *s3.GetBucketEncryptionOutput
	var lifecycleData *s3.GetBucketLifecycleConfigurationOutput

	if r := <-policyCh; r.err != nil {
		definition.SetPolicyAPIError(r.err)
	} else {
		definition.SetPolicy(r.data)
		policyData = r.data
	}
	if r := <-versionCh; r.err != nil {
		definition.SetVersionAPIError(r.err)
	} else {
		definition.SetVersion(r.data)
		versionData = r.data
	}
	if r := <-tagsCh; r.err != nil {
		definition.SetTagsAPIError(r.err)
	} else {
		definition.SetTags(r.data)
		tagsData = r.data
	}
	if r := <-encryptionCh; r.err != nil {
		definition.SetEncryptionConfigAPIError(r.err)
	} else {
		definition.SetEncryptionConfig(r.data)
		encryptionData = r.data
	}
	if r := <-lifecycleCh; r.err != nil {
		definition.SetLifeCycleError(r.err)
	} else {
		definition.SetLifeCycle(r.data)
		lifecycleData = r.data
	}

	// AI narration is additive: a failed or unreachable Summarize call never
	// fails Fetch itself, it only leaves aiSummaryUnavailable set (ADR-010).
	facts := bucketDefinitionEvidence(f.bucketName, policyData, versionData, tagsData, encryptionData, lifecycleData)
	definition.applyAISummary(ctx, ai.NewClientFromEnv(), facts)

	return definition, nil
}

// applyAISummary sets aiSummary or aiSummaryUnavailable from the given
// evidence, never returning an error — a failed or unreachable Summarize
// call must never fail the surrounding Fetch (ADR-010). Split out from
// Fetch so this exact integration point (evidence in, AI client behavior,
// bucketDefinition fields out) is directly testable without a real AWS call.
func (definition *bucketDefinition) applyAISummary(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	log.Printf("Evidence, %v", facts)
	if len(facts) == 0 {
		definition.SetAISummaryUnavailable("no evidence could be gathered (all fetches failed)")
		return
	}
	summary, err := client.Summarize(ctx, facts)
	if err != nil {
		definition.SetAISummaryUnavailable(err.Error())
		return
	}
	definition.SetAISummary(summary)
}

func (f bucketObjectsDownloadFetcher) Fetch(ctx context.Context) (*bucketOjectsDownloadSummary, error) {
	objectDownloadSummaryChan := make(chan *objectDownloadSummary, 100) // Buffered channel
	objectsDownloadSummary := []*objectDownloadSummary{}

	// Ensure channel is closed after all operations complete
	defer close(objectDownloadSummaryChan)

	if f.recursive {
		input := &s3.ListObjectsInput{}
		input.Bucket = &f.bucketName
		input.Prefix = &f.key
		apiOutput, err := f.client.ListObjects(ctx, input)
		if err != nil {
			return nil, aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil)
		}
		if len(apiOutput.Contents) == 0 {
			return nil, aws.NewErrorInfo(NoObjectFoundWithGivenPrefix(f.bucketName, f.key), viewer.WARN, nil)
		}

		// Use WaitGroup to ensure all downloads complete
		var wg sync.WaitGroup
		wg.Add(len(apiOutput.Contents))

		// Launch download goroutines
		for _, object := range apiOutput.Contents {
			go func(objKey string) {
				defer wg.Done()
				downloadObject(ctx, f.bucketName, objKey, f.path, f.downloader, objectDownloadSummaryChan)
			}(*object.Key)
		}

		// Wait for all downloads to complete
		wg.Wait()

		// Collect all results, bounded by one shared 30s deadline for the
		// whole collection phase (not 30s per remaining item).
		collectDeadline := time.After(30 * time.Second)
	collectLoop:
		for i := 0; i < len(apiOutput.Contents); i++ {
			select {
			case summary := <-objectDownloadSummaryChan:
				objectsDownloadSummary = append(objectsDownloadSummary, summary)
			case <-collectDeadline:
				break collectLoop
			}
		}

		return &bucketOjectsDownloadSummary{bucketName: f.bucketName, objectsDownloadSummary: objectsDownloadSummary}, nil
	}

	// Single object download
	go downloadObject(ctx, f.bucketName, f.key, f.path, f.downloader, objectDownloadSummaryChan)

	select {
	case summary := <-objectDownloadSummaryChan:
		objectsDownloadSummary = append(objectsDownloadSummary, summary)
	case <-time.After(30 * time.Second): // Timeout protection
		return nil, aws.NewErrorInfo(fmt.Errorf("download timeout"), viewer.ERROR, nil)
	}

	return &bucketOjectsDownloadSummary{bucketName: f.bucketName, objectsDownloadSummary: objectsDownloadSummary}, nil
}

func downloadObject(ctx context.Context, bucketName, key, path string, downloader *manager.Downloader, downloadSummaryChan chan<- *objectDownloadSummary) {
	start := time.Now()
	downloadFileAbsPath := fmt.Sprintf("%s/%s", path, key)

	fileDir := filepath.Dir(downloadFileAbsPath)

	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		err := os.MkdirAll(fileDir, os.ModePerm)
		if err != nil {
			fmt.Println("error occurred during create dir ", err)
		}
	}

	file, err := os.Create(downloadFileAbsPath)
	if err != nil {
		if file != nil {
			file.Close()
		}
		downloadSummaryChan <- newBucketObjectDownloadSummary(key, "", 0, time.Since(start), aws.NewErrorInfo(err, viewer.ERROR, nil))
		return
	}

	// Ensure file is closed after download
	defer file.Close()

	numBytesWrite, err := downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		downloadSummaryChan <- newBucketObjectDownloadSummary(key, "", 0, time.Since(start), aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil))
	} else {
		downloadSummaryChan <- newBucketObjectDownloadSummary(key, file.Name(), numBytesWrite, time.Since(start), nil)
	}
}

func getBucketPolicy(ctx context.Context, bucket *string, client bucketConfigurationAPI) (*s3.GetBucketPolicyOutput, error) {
	res, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: bucket})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			fmt.Println("errCode", apiErr.ErrorCode(), " message ", apiErr.ErrorMessage())
		}
		return nil, err
	}
	return res, nil
}

func getBucketVersionConfig(ctx context.Context, bucket *string, client bucketConfigurationAPI) (*s3.GetBucketVersioningOutput, error) {
	res, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: bucket})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getBucketTags(ctx context.Context, bucket *string, client bucketConfigurationAPI) (*s3.GetBucketTaggingOutput, error) {
	res, err := client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: bucket})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getBucketencryptionConfig(ctx context.Context, bucket *string, client bucketConfigurationAPI) (*s3.GetBucketEncryptionOutput, error) {
	res, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: bucket})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getBucketLifecycleConfig(ctx context.Context, bucket *string, client bucketConfigurationAPI) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	res, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: bucket})

	if err != nil {
		fmt.Println("Error message", err.Error())
		return nil, err
	}
	return res, nil
}
