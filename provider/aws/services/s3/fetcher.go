package s3

import (
	"cloudctl/provider/aws"
	itime "cloudctl/time"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

var bucketInfoRetrieverFuncs = []func(bucket *string, client *aws.Client, bucketinfo *bucketDefinition, wg *sync.WaitGroup){
	getBucketPolicy,
	getBucketVersionConfig,
	getBucketTags,
	getBucketencryptionConfig,
	getBucketLifecycleConfig,
}

type bucketListFetcher struct {
	client *aws.Client
	filter *bucketListFilter
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
		return &bucketListOutput{err: &aws.APIException{}}
	}
	if len(apiOutput.Buckets) == 0 {
		return &bucketListOutput{err: NoBucketFound{}}
	}
	buckets := []*bucket{}
	for _, o := range apiOutput.Buckets {
		if f.filter.Apply(o) {
			buckets = append(buckets, &bucket{
				name:         o.Name,
				creationDate: f.tz.AdaptTimezone(o.CreationDate),
			})
		}
	}
	// default sort(asc) by creation darte
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].creationDate.Before(*buckets[j].creationDate)
	})
	return &bucketListOutput{buckets: buckets, err: nil}
}

func (f bucketObjectsFetcher) Fetch() interface{} {
	output := []*object{}

	objectsPtr, err := fetchBucketObjects(f.bucketName, f.objectPrefix, f.client)
	if err != nil {
		return &bucketObjectListOutput{err: &aws.APIException{}} // TODO : handle specific error
	}

	for _, o := range *objectsPtr {
		output = append(output, &object{
			key:          o.Key,
			sizeInBytes:  o.Size,
			storageClass: o.StorageClass,
			lastModified: f.tz.AdaptTimezone(o.LastModified),
		})
	}
	return &bucketObjectListOutput{bucketName: &f.bucketName, objects: output, err: nil}
}

func (f bucketConfigurationFetcher) Fetch() interface{} {

	definition := &bucketDefinition{}
	definition.SetBucketName(f.bucketName)

	wg := new(sync.WaitGroup)
	wg.Add(len(bucketInfoRetrieverFuncs))

	for _, function := range bucketInfoRetrieverFuncs {
		go function(&f.bucketName, f.client, definition, wg)
	}
	wg.Wait()
	return definition
}

func (f bucketObjectsDownloadFetcher) Fetch() interface{} {
	downloadSummaryChan := make(chan *objectDownloadSummary)
	downloadSummaries := []*objectDownloadSummary{}
	defer close(downloadSummaryChan)
	if f.recursive {
		input := &s3.ListObjectsInput{}
		input.Bucket = &f.bucketName
		input.Prefix = &f.key
		listBucketObjects, err := f.client.S3.ListObjects(input)
		if err != nil {
			return &bucketOjectsDownloadSummary{err: &aws.APIException{}} // TODO : handle specific error
		}

		for _, object := range listBucketObjects.Contents {
			go downloadObject(f.bucketName, *object.Key, f.path, f.client, downloadSummaryChan)
		}
		for i := 0; i < len(listBucketObjects.Contents); i++ {
			downloadSummaries = append(downloadSummaries, <-downloadSummaryChan)
		}
		return &bucketOjectsDownloadSummary{bucketName: f.bucketName, downloadSummaries: downloadSummaries, err: nil}
	}
	go downloadObject(f.bucketName, f.key, f.path, f.client, downloadSummaryChan)
	downloadSummaries = append(downloadSummaries, <-downloadSummaryChan)
	return &bucketOjectsDownloadSummary{bucketName: f.bucketName, downloadSummaries: downloadSummaries, err: nil}
}

func fetchBucketObjects(bucketName, objectPrefix string, client *aws.Client) (*[]*s3.Object, error) {

	var recursiveFetch func(bucketName, objectPrefix string, objectsPtr *[]*s3.Object, nextMarket string, client *aws.Client) error

	recursiveFetch = func(bucketName, objectPrefix string, objectsPtr *[]*s3.Object, nextMarker string, client *aws.Client) error {
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
			recursiveFetch(bucketName, objectPrefix, objectsPtr, nextMarker, client)
		}
		return nil
	}
	objects := []*s3.Object{}
	recursiveFetch(bucketName, objectPrefix, &objects, "", client)
	return &objects, nil
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
	defer file.Close()
	if err != nil {
		downloadSummaryChan <- &objectDownloadSummary{err: err}
	} else {
		numBytesWrite, err := client.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: &bucketName,
			Key:    &key,
		})
		if err != nil {
			downloadSummaryChan <- &objectDownloadSummary{err: err}
		} else {
			downloadSummaryChan <- &objectDownloadSummary{
				source:      key,
				destination: file.Name(),
				sizeinBytes: numBytesWrite,
				timeElapsed: time.Since(start),
			}

		}
	}
}

func getBucketPolicy(bucket *string, client *aws.Client, bucketinfo *bucketDefinition, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.S3.GetBucketPolicy(&s3.GetBucketPolicyInput{Bucket: bucket})
	if err != nil {
		errr, _ := err.(awserr.Error)
		fmt.Println("errCode", errr.Code(), " message ", errr.Message())
		bucketinfo.SetPolicyAPIError(err)
		return
	}
	bucketinfo.SetPolicy(res)
}

func getBucketVersionConfig(bucket *string, client *aws.Client, bucketinfo *bucketDefinition, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.S3.GetBucketVersioning(&s3.GetBucketVersioningInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetVersionAPIError(err)
		return
	}
	bucketinfo.SetVersion(res)
}

func getBucketTags(bucket *string, client *aws.Client, bucketinfo *bucketDefinition, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.S3.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetTagsAPIError(err)
		return
	}
	bucketinfo.SetTags(res)
}

func getBucketencryptionConfig(bucket *string, client *aws.Client, bucketinfo *bucketDefinition, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.S3.GetBucketEncryption(&s3.GetBucketEncryptionInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetEncryptionConfigAPIError(err)
		return
	}
	bucketinfo.SetEncryptionConfig(res)
}

func getBucketLifecycleConfig(bucket *string, client *aws.Client, bucketinfo *bucketDefinition, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.S3.GetBucketLifecycleConfiguration(&s3.GetBucketLifecycleConfigurationInput{Bucket: bucket})

	if err != nil {
		fmt.Println("Error message", err.Error())
		bucketinfo.SetLifeCycleError(err)
		return
	}
	bucketinfo.SetLifeCycle(res)
}
