package s3

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	itime "cloudctl/time"
	"cloudctl/viewer"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/sync/errgroup"
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
	client     listObjectsAPI
	downloader *manager.Downloader
	bucketName string
	key        string
	path       string
	recursive  bool
}

// maxConcurrentObjectDownloads bounds how many objects download at once in
// the recursive path. Unbounded fan-out here would open one goroutine, one
// file handle, and one network connection per object simultaneously — for a
// prefix with thousands of objects that's a real resource-exhaustion risk.
// Bounded to the same order of magnitude as the other bounded fan-out sites
// in this codebase (CloudWatch stats @8, IAM policy checks @5) — see
// ADR-0018.
const maxConcurrentObjectDownloads = 8

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
	definition.applyAINarration(ctx, ai.NewClientFromEnv(), facts)

	return definition, nil
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error — a failed or unreachable Summarize/Recommend call must never fail
// the surrounding Fetch (ADR-010). Summarize and Recommend run concurrently
// under a single spinner rather than back-to-back, mirroring dynamodb's
// applyAINarration. Split out from Fetch so this exact integration point
// (evidence in, AI client behavior, bucketDefinition fields out) is
// directly testable without a real AWS call.
func (definition *bucketDefinition) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		definition.SetAISummaryUnavailable("no evidence could be gathered (all fetches failed)")
		definition.SetAIRecommendationsUnavailable("no evidence could be gathered (all fetches failed)")
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})

	if result.summaryErr != "" {
		definition.SetAISummaryUnavailable(result.summaryErr)
	} else {
		definition.SetAISummary(result.summary)
	}
	if result.recommendedErr != "" {
		definition.SetAIRecommendationsUnavailable(result.recommendedErr)
	} else {
		definition.SetAIRecommendations(result.recommendations)
	}
}

func (f bucketObjectsDownloadFetcher) Fetch(ctx context.Context) (*bucketOjectsDownloadSummary, error) {
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

		// Bounded fan-out (ADR-0018): each goroutine writes its own
		// pre-assigned slice index, so there's no data race despite no
		// mutex, and no channel/collection-deadline is needed — overall
		// wait time is naturally bounded by the semaphore instead of an
		// arbitrary fixed timeout. Mirrors statisticsFetcher.Fetch and
		// bucketImpactFetcher.Fetch.
		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, maxConcurrentObjectDownloads)
		summaries := make([]*objectDownloadSummary, len(apiOutput.Contents))
		for i, object := range apiOutput.Contents {
			i, objKey := i, *object.Key
			g.Go(func() error {
				sem <- struct{}{}
				defer func() { <-sem }()
				summaries[i] = downloadObjectResult(gCtx, f.bucketName, objKey, f.path, f.downloader)
				return nil
			})
		}
		_ = g.Wait() // per-object errors are carried in each summary's err field, not fatal to the batch

		return &bucketOjectsDownloadSummary{bucketName: f.bucketName, objectsDownloadSummary: summaries}, nil
	}

	// Single object download — no fan-out, unaffected by ADR-0018.
	objectDownloadSummaryChan := make(chan *objectDownloadSummary, 1)
	go downloadObject(ctx, f.bucketName, f.key, f.path, f.downloader, objectDownloadSummaryChan)

	select {
	case summary := <-objectDownloadSummaryChan:
		return &bucketOjectsDownloadSummary{bucketName: f.bucketName, objectsDownloadSummary: []*objectDownloadSummary{summary}}, nil
	case <-time.After(30 * time.Second): // Timeout protection
		return nil, aws.NewErrorInfo(fmt.Errorf("download timeout"), viewer.ERROR, nil)
	}
}

// downloadObjectResult downloads one object and returns its summary
// directly — the synchronous core shared by both the single-object path
// (via the downloadObject channel wrapper below) and the bounded recursive
// fan-out, which writes results straight into a pre-sized slice instead of
// coordinating through a channel.
func downloadObjectResult(ctx context.Context, bucketName, key, path string, downloader *manager.Downloader) *objectDownloadSummary {
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
		return newBucketObjectDownloadSummary(key, "", 0, time.Since(start), aws.NewErrorInfo(err, viewer.ERROR, nil))
	}

	// Ensure file is closed after download
	defer file.Close()

	numBytesWrite, err := downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		return newBucketObjectDownloadSummary(key, "", 0, time.Since(start), aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil))
	}
	return newBucketObjectDownloadSummary(key, file.Name(), numBytesWrite, time.Since(start), nil)
}

// downloadObject wraps downloadObjectResult for the single-object download
// path, which coordinates via a channel rather than a pre-sized slice.
func downloadObject(ctx context.Context, bucketName, key, path string, downloader *manager.Downloader, downloadSummaryChan chan<- *objectDownloadSummary) {
	downloadSummaryChan <- downloadObjectResult(ctx, bucketName, key, path, downloader)
}

func getBucketPolicy(ctx context.Context, bucket *string, client bucketConfigurationAPI) (*s3.GetBucketPolicyOutput, error) {
	res, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: bucket})
	if err != nil {
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
		return nil, err
	}
	return res, nil
}
