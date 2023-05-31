package s3

import (
	itime "cloudctl/time"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var bucketInfoRetrieverFuncs = []func(bucket *string, client *s3.S3, bucketinfo *bucketInfo, wg *sync.WaitGroup){
	getBucketPolicy,
	getBucketVersionConfig,
	getBucketTags,
	getBucketencryptionConfig,
	getBucketLifecycleConfig,
}

type bucketListFetcher struct {
	client *s3.S3
	filter *bucketListFilter
	tz     *itime.Timezone
}

type bucketObjectsFetcher struct {
	client *s3.S3
	// fetch all objects for provided bucket
	bucketName   string
	objectPrefix string
	tz           *itime.Timezone
}

type bucketConfigurationFetcher struct {
	client *s3.S3
	// fetch configuration for provided bucket
	bucketName string
}
type bucketObjectsDownloadFetcher struct {
	client     *s3.S3
	downloader *s3manager.Downloader
	bucketName string
	key        string
	path       string
	recursive  bool
}

func (f bucketListFetcher) Fetch() (interface{}, error) {

	// fmt.Println(*f.filter.creationDateFrom, " - ", *f.filter.creationDateTo)

	buckets, err := f.client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	op := []*bucket{}

	for _, o := range buckets.Buckets {
		if f.filter.Apply(o, f.tz) {
			op = append(op, &bucket{
				name:         o.Name,
				creationDate: f.tz.AdaptTimezone(o.CreationDate),
			})
		}
	}
	// default sort(asc) by creation darte
	sort.Slice(op, func(i, j int) bool {
		return op[i].creationDate.Before(*op[j].creationDate)
	})
	return &bucketListOutput{buckets: op}, nil
}

func (f bucketObjectsFetcher) Fetch() (interface{}, error) {

	input := &s3.ListObjectsInput{}
	input.Bucket = &f.bucketName
	input.Prefix = &f.objectPrefix
	listBucketObjects, err := f.client.ListObjects(input)
	if err != nil {
		return nil, err
	}

	objects := []*object{}

	for _, content := range listBucketObjects.Contents {
		objects = append(objects, &object{
			key:          content.Key,
			sizeInBytes:  content.Size,
			storageClass: content.StorageClass,
			lastModified: f.tz.AdaptTimezone(content.LastModified),
		})
	}
	return &bucketObjectListOutput{bucketName: &f.bucketName, objects: objects}, nil
}

func (f bucketConfigurationFetcher) Fetch() (interface{}, error) {

	bucketInfo := &bucketInfo{}
	bucketInfo.SetBucketName(f.bucketName)

	wg := new(sync.WaitGroup)
	wg.Add(len(bucketInfoRetrieverFuncs))

	for _, function := range bucketInfoRetrieverFuncs {
		go function(&f.bucketName, f.client, bucketInfo, wg)
	}
	wg.Wait()
	return bucketInfo, nil
}

func (f bucketObjectsDownloadFetcher) Fetch() (interface{}, error) {
	downloadSummaryChan := make(chan *bucketObjectsDownloadSummary)
	downloadSummaries := []*bucketObjectsDownloadSummary{}
	defer close(downloadSummaryChan)
	if f.recursive {
		input := &s3.ListObjectsInput{}
		input.Bucket = &f.bucketName
		input.Prefix = &f.key
		listBucketObjects, err := f.client.ListObjects(input)
		if err != nil {
			// TODO : handle error
			fmt.Println("error occur duing list of objects")
			return nil, err
		}

		for _, object := range listBucketObjects.Contents {
			go downloadObject(f.bucketName, *object.Key, f.path, f.downloader, downloadSummaryChan)
		}
		for i := 0; i < len(listBucketObjects.Contents); i++ {
			downloadSummaries = append(downloadSummaries, <-downloadSummaryChan)
		}
		return downloadSummaries, nil
	}
	go downloadObject(f.bucketName, f.key, f.path, f.downloader, downloadSummaryChan)
	downloadSummaries = append(downloadSummaries, <-downloadSummaryChan)
	return downloadSummaries, nil
}

func downloadObject(bucketName, key, path string, downloaderClient *s3manager.Downloader, downloadSummaryChan chan<- *bucketObjectsDownloadSummary) {
	start := time.Now()
	downloadFileAbsPath := fmt.Sprintf("%s/%s", path, key)

	fileDir := filepath.Dir(downloadFileAbsPath)

	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		err := os.MkdirAll(fileDir, os.ModePerm)
		if err != nil {
			//TODO: handle error
			fmt.Println("erro occur during create dir ", err)
		}
	}

	file, err := os.Create(downloadFileAbsPath)
	if err != nil {
		//TODO: handle error
		fmt.Println("[file] err occur on ", key, err)
	}
	defer file.Close()

	numBytesWrite, err := downloaderClient.Download(file, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		//TODO: handle error
		fmt.Println("[numBytesWrite] err occur on ", key)
	}

	downloadSummaryChan <- &bucketObjectsDownloadSummary{
		source:      key,
		destination: file.Name(),
		sizeinBytes: numBytesWrite,
		timeElapsed: time.Since(start),
	}
}

func getBucketPolicy(bucket *string, client *s3.S3, bucketinfo *bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.GetBucketPolicy(&s3.GetBucketPolicyInput{Bucket: bucket})
	if err != nil {
		errr, _ := err.(awserr.Error)
		fmt.Println("errCode", errr.Code(), " message ", errr.Message())
		bucketinfo.SetPolicyAPIError(err)
		return
	}
	bucketinfo.SetPolicy(res)
}

func getBucketVersionConfig(bucket *string, client *s3.S3, bucketinfo *bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.GetBucketVersioning(&s3.GetBucketVersioningInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetVersionAPIError(err)
		return
	}
	bucketinfo.SetVersion(res)
}

func getBucketTags(bucket *string, client *s3.S3, bucketinfo *bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetTagsAPIError(err)
		return
	}
	bucketinfo.SetTags(res)
}

func getBucketencryptionConfig(bucket *string, client *s3.S3, bucketinfo *bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.GetBucketEncryption(&s3.GetBucketEncryptionInput{Bucket: bucket})
	if err != nil {
		bucketinfo.SetEncryptionConfigAPIError(err)
		return
	}
	bucketinfo.SetEncryptionConfig(res)
}

func getBucketLifecycleConfig(bucket *string, client *s3.S3, bucketinfo *bucketInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	res, err := client.GetBucketLifecycleConfiguration(&s3.GetBucketLifecycleConfigurationInput{Bucket: bucket})

	if err != nil {
		fmt.Println("Error message", err.Error())
		bucketinfo.SetLifeCycleError(err)
		return
	}
	bucketinfo.SetLifeCycle(res)
}
