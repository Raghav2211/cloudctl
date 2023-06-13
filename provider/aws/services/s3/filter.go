package s3

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type BucketListFilterOptFunc func(*BucketListFilter)

type BucketListFilter struct {
	creationDateString *string
	bucketNameString   *string
}

func (f *BucketListFilter) applyCustomFilter(bucket *s3.Bucket) bool {
	if f.bucketNameString == nil && f.creationDateString == nil {
		return true
	}
	if f.bucketNameString != nil && f.creationDateString != nil {
		return bucketNameFilter(*bucket.Name, *f.bucketNameString) && creationDateFilter(*bucket.CreationDate, *f.creationDateString)
	}
	if f.bucketNameString != nil {
		return bucketNameFilter(*bucket.Name, *f.bucketNameString)
	}
	if f.creationDateString != nil {
		return creationDateFilter(*bucket.CreationDate, *f.creationDateString)
	}
	return false
}

func NewBucketListFilter(optFuncs ...BucketListFilterOptFunc) *BucketListFilter {
	filter := &BucketListFilter{
		creationDateString: nil,
		bucketNameString:   nil,
	}
	for _, optFunc := range optFuncs {
		optFunc(filter)
	}
	return filter
}

func WithCreationDateFilter(creationDateString *string) BucketListFilterOptFunc {
	return func(blf *BucketListFilter) {
		if creationDateString != nil {
			blf.creationDateString = creationDateString
		}
	}
}

func WithBucketNameFilter(bucketNameString *string) BucketListFilterOptFunc {
	return func(blf *BucketListFilter) {
		if bucketNameString != nil {
			blf.bucketNameString = bucketNameString
		}
	}
}

func bucketNameFilter(bucketNameFromAPI, bucketNameFromCLI string) bool {
	return strings.Contains(bucketNameFromAPI, bucketNameFromCLI)
}

func creationDateFilter(dateFromAPI time.Time, creationDateInString string) bool {

	isLastCharWildCard := strings.Index(creationDateInString, "*") == len(creationDateInString)-1
	// convert bucket created time to ISO-8601
	bucketCreatedTime := dateFromAPI.Format(time.RFC3339)
	if isLastCharWildCard {
		return strings.Contains(bucketCreatedTime, creationDateInString[:len(creationDateInString)-1])
	} else {
		return strings.Contains(bucketCreatedTime, creationDateInString)
	}
}
