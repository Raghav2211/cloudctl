package s3

import (
	"cloudctl/provider/aws"
	itime "cloudctl/time"
	"cloudctl/viewer"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

type bucketListFetcher struct {
	client *aws.Client
	filter *BucketListFilter
	tz     *itime.Timezone
}

type bucketObjectsFetcher struct {
	client *aws.Client
	// fetch all objects for provided bucket
	bucketName   string
	objectPrefix string
	tz           *itime.Timezone
}

type bucketConfigurationFetcher struct {
	client *aws.Client
	// fetch configuration for provided bucket
	bucketName string
}
type bucketObjectsDownloadFetcher struct {
	client     *aws.Client
	bucketName string
	key        string
	path       string
	recursive  bool
}

func (f bucketListFetcher) Fetch() interface{} {

	apiOutput, err := f.client.S3.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		errorInfo := aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil)
		return &bucketListOutput{err: errorInfo}
	}
	if len(apiOutput.Buckets) == 0 {
		errorInfo := &aws.ErrorInfo{Err: NoBucketFound(), ErrorType: viewer.INFO}
		return &bucketListOutput{err: errorInfo}
	}
	buckets := []*bucketOutput{}
	for _, o := range apiOutput.Buckets {
		if f.filter.applyCustomFilter(o) {
			buckets = append(buckets, newBucketOutput(o, f.tz))
		}
	}
	// default sort(asc) by creation darte
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].creationDate.Before(*buckets[j].creationDate)
	})
	return &bucketListOutput{buckets: buckets}
}

func (f bucketObjectsFetcher) Fetch() interface{} {
	output := []*bucketObjectOutput{}

	objectsPtr, err := fetchBucketObjects(f.bucketName, f.objectPrefix, f.client)

	for _, o := range *objectsPtr {
		output = append(output, newBucketObjectOutput(o, f.tz))
	}
	if err != nil {
		errorInfo := aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil)
		return &bucketObjectListOutput{err: errorInfo}
	}
	if len(output) == 0 {
		errorInfo := aws.NewErrorInfo(NoObjectFound(f.bucketName), viewer.INFO, nil)
		return &bucketObjectListOutput{err: errorInfo}
	}
	return &bucketObjectListOutput{bucketName: &f.bucketName, objects: output}
}

func (f bucketConfigurationFetcher) Fetch() interface{} {

	definition := &bucketDefinition{}
	definition.SetBucketName(f.bucketName)

	wg := new(sync.WaitGroup)
	wg.Add(5)

	go func() {
		defer wg.Done()
		data := getBucketPolicy(&f.bucketName, f.client, definition)
		if data != nil {
			definition.SetPolicy(data)
		}
	}()
	go func() {
		defer wg.Done()
		data := getBucketVersionConfig(&f.bucketName, f.client, definition)
		if data != nil {
			definition.SetVersion(data)
		}
	}()
	go func() {
		defer wg.Done()
		data := getBucketTags(&f.bucketName, f.client, definition)
		if data != nil {
			definition.SetTags(data)
		}
	}()
	go func() {
		defer wg.Done()
		data := getBucketencryptionConfig(&f.bucketName, f.client, definition)
		if data != nil {
			definition.SetEncryptionConfig(data)
		}
	}()
	go func() {
		defer wg.Done()
		data := getBucketLifecycleConfig(&f.bucketName, f.client, definition)
		if data != nil {
			definition.SetLifeCycle(data)
		}
	}()
	wg.Wait()
	return definition
}

func (f bucketObjectsDownloadFetcher) Fetch() interface{} {
	objectDownloadSummaryChan := make(chan *objectDownloadSummary)
	objectsDownloadSummary := []*objectDownloadSummary{}
	defer close(objectDownloadSummaryChan)
	if f.recursive {
		input := &s3.ListObjectsInput{}
		input.Bucket = &f.bucketName
		input.Prefix = &f.key
		apiOutput, err := f.client.S3.ListObjects(input)
		if err != nil {
			return &bucketOjectsDownloadSummary{err: aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil)}
		}
		if len(apiOutput.Contents) == 0 {
			return &bucketOjectsDownloadSummary{err: aws.NewErrorInfo(NoObjectFoundWithGivenPrefix(f.bucketName, f.key), viewer.WARN, nil)}
		}

		for _, object := range apiOutput.Contents {
			go downloadObject(f.bucketName, *object.Key, f.path, f.client, objectDownloadSummaryChan)
		}
		for i := 0; i < len(apiOutput.Contents); i++ {
			objectsDownloadSummary = append(objectsDownloadSummary, <-objectDownloadSummaryChan)
		}
		return &bucketOjectsDownloadSummary{bucketName: f.bucketName, objectsDownloadSummary: objectsDownloadSummary, err: nil}
	}
	go downloadObject(f.bucketName, f.key, f.path, f.client, objectDownloadSummaryChan)
	objectsDownloadSummary = append(objectsDownloadSummary, <-objectDownloadSummaryChan)
	return &bucketOjectsDownloadSummary{bucketName: f.bucketName, objectsDownloadSummary: objectsDownloadSummary, err: nil}
}

func fetchBucketObjects(bucketName, objectPrefix string, client *aws.Client) (*[]*s3.Object, error) {

	var fetch func(bucketName, objectPrefix string, objectsPtr *[]*s3.Object, nextMarket string, client *aws.Client) error

	fetch = func(bucketName, objectPrefix string, objectsPtr *[]*s3.Object, nextMarker string, client *aws.Client) error {
		input := &s3.ListObjectsInput{}
		input.Bucket = &bucketName
		input.Prefix = &objectPrefix
		if nextMarker != "" {
			input.Marker = &nextMarker
		}

		apiOutput, err := client.S3.ListObjects(input)
		if err != nil {
			return err
		}
		*objectsPtr = append(*objectsPtr, apiOutput.Contents...)
		if *apiOutput.IsTruncated {
			nextMarker = *apiOutput.Contents[len(apiOutput.Contents)-1].Key
			fetch(bucketName, objectPrefix, objectsPtr, nextMarker, client)
		}
		return nil
	}
	objects := []*s3.Object{}
	nextMarker := "" // empty marker
	err := fetch(bucketName, objectPrefix, &objects, nextMarker, client)
	return &objects, err
}

func downloadObject(bucketName, key, path string, client *aws.Client, downloadSummaryChan chan<- *objectDownloadSummary) {
	start := time.Now()
	downloadFileAbsPath := fmt.Sprintf("%s/%s", path, key)

	fileDir := filepath.Dir(downloadFileAbsPath)

	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		err := os.MkdirAll(fileDir, os.ModePerm)
		if err != nil {
			fmt.Println("erro occur during create dir ", err)
		}
	}

	file, err := os.Create(downloadFileAbsPath)
	if err != nil {
		defer file.Close()
		downloadSummaryChan <- newBucketObjectDownloadSummary(key, "", 0, time.Since(start), aws.NewErrorInfo(err, viewer.ERROR, nil))
	} else {
		numBytesWrite, err := client.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: &bucketName,
			Key:    &key,
		})
		if err != nil {
			downloadSummaryChan <- newBucketObjectDownloadSummary(key, "", 0, time.Since(start), aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil))
		} else {
			downloadSummaryChan <- newBucketObjectDownloadSummary(key, file.Name(), numBytesWrite, time.Since(start), nil)
		}
	}
}

func getBucketPolicy(bucket *string, client *aws.Client, bucketinfo *bucketDefinition) *s3.GetBucketPolicyOutput {
	res, err := client.S3.GetBucketPolicy(&s3.GetBucketPolicyInput{Bucket: bucket})
	if err != nil {
		errr, _ := err.(awserr.Error)
		fmt.Println("errCode", errr.Code(), " message ", errr.Message())
		bucketinfo.SetPolicyAPIError(err)
		return nil
	}
	return res
}

func getBucketVersionConfig(bucket *string, client *aws.Client, bucketinfo *bucketDefinition) *s3.GetBucketVersioningOutput {
	res, err := client.S3.GetBucketVersioning(&s3.GetBucketVersioningInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetVersionAPIError(err)
		return nil
	}
	return res
}

func getBucketTags(bucket *string, client *aws.Client, bucketinfo *bucketDefinition) *s3.GetBucketTaggingOutput {
	res, err := client.S3.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetTagsAPIError(err)
		return nil
	}
	return res
}

func getBucketencryptionConfig(bucket *string, client *aws.Client, bucketinfo *bucketDefinition) *s3.GetBucketEncryptionOutput {
	res, err := client.S3.GetBucketEncryption(&s3.GetBucketEncryptionInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetEncryptionConfigAPIError(err)
		return nil
	}
	return res
}

func getBucketLifecycleConfig(bucket *string, client *aws.Client, bucketinfo *bucketDefinition) *s3.GetBucketLifecycleConfigurationOutput {
	res, err := client.S3.GetBucketLifecycleConfiguration(&s3.GetBucketLifecycleConfigurationInput{Bucket: bucket})

	if err != nil {
		fmt.Println("Error message", err.Error())
		bucketinfo.SetLifeCycleError(err)
		return nil
	}
	return res
}
