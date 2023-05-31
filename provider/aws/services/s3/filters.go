package s3

import (
	itime "cloudctl/time"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type bucketListFilter struct {
	creationDateFrom *time.Time
	creationDateTo   *time.Time
}

func (f *bucketListFilter) Apply(bucket *s3.Bucket, tz *itime.Timezone) bool {
	creationDateWithTz := tz.AdaptTimezone(bucket.CreationDate) // TODO :this should be done by producer, remove tz paramater
	return f.filterByCreationDateFrom(creationDateWithTz) && f.filterByCreationDateTo(creationDateWithTz)
}

func (f *bucketListFilter) filterByCreationDateFrom(creationDate *time.Time) bool {
	return f.creationDateFrom.IsZero() || f.creationDateFrom.Equal(*creationDate) || f.creationDateFrom.Before(*creationDate)
}

func (f *bucketListFilter) filterByCreationDateTo(creationDate *time.Time) bool {
	return f.creationDateTo.IsZero() || f.creationDateTo.Equal(*creationDate) || f.creationDateTo.After(*creationDate)
}
