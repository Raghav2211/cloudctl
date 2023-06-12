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
	if f.bucketNameString != nil && strings.Contains(*bucket.Name, *f.bucketNameString) {
		return true
	}
	if f.creationDateString != nil {
		isLastCharWildCard := strings.Index(*f.creationDateString, "*") == len(*f.creationDateString)-1
		if isLastCharWildCard {
			creationDateWithwildCard := *f.creationDateString
			// convert bucket created time to ISO-8601
			bucketCreatedTime := bucket.CreationDate.Format(time.RFC3339)
			return strings.Contains(bucketCreatedTime, creationDateWithwildCard[:len(creationDateWithwildCard)-1])
		} else {
			return strings.Contains(bucket.CreationDate.GoString(), *f.creationDateString)
		}
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

func WithCreationDateFilter(creationDateString string) BucketListFilterOptFunc {
	return func(blf *BucketListFilter) {
		blf.creationDateString = &creationDateString
	}
}

func WithBucketNameFilter(bucketNameString string) BucketListFilterOptFunc {
	return func(blf *BucketListFilter) {
		blf.bucketNameString = &bucketNameString
	}
}
