package s3

import (
	itime "cloudctl/time"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

//TODO: creatio of filters more dynamically : initial thought :- can use a queue which contains condition with operator
//and downstream filter only apply this filter

// type filter struct {
// 	conditions , operators
// }

type bucketListFilter struct {
	creationDateFrom *time.Time
	creationDateTo   *time.Time
	bucketNameString *string
	tz               *itime.Timezone
}

func (f *bucketListFilter) String() string {
	fromFilter := "N/A"
	toFilter := "N/A"
	if !f.creationDateFrom.IsZero() {
		fromFilter = f.creationDateFrom.String()
	}
	if !f.creationDateTo.IsZero() {
		toFilter = f.creationDateTo.String()
	}

	return fmt.Sprintf("[from=%s , to=%s , bucketPrefix=%s ]", fromFilter, toFilter, *f.bucketNameString)
}

func (f *bucketListFilter) Apply(bucket *s3.Bucket) bool {

	isCreationDateFromFilterOff := f.creationDateFrom.IsZero()
	isCreationDateToFilterOff := f.creationDateTo.IsZero()
	isBucketNameFilterOff := f.bucketNameString == nil || *f.bucketNameString == ""

	if isCreationDateFromFilterOff && isCreationDateToFilterOff && isBucketNameFilterOff {
		return true
	}
	if !isCreationDateFromFilterOff || !isCreationDateToFilterOff {
		creationDateWithTz := f.tz.AdaptTimezone(bucket.CreationDate)
		filterByTime := (f.filterByCreationDateFrom(creationDateWithTz) && f.filterByCreationDateTo(creationDateWithTz))
		if isBucketNameFilterOff {
			return filterByTime
		}
		return filterByTime && f.filterByNameStringContains(bucket.Name)
	}
	if !isBucketNameFilterOff {
		return f.filterByNameStringContains(bucket.Name)
	}
	return true
}

func (f *bucketListFilter) filterByCreationDateFrom(creationDate *time.Time) bool {
	return f.creationDateFrom.IsZero() || f.creationDateFrom.Equal(*creationDate) || f.creationDateFrom.Before(*creationDate)
}

func (f *bucketListFilter) filterByCreationDateTo(creationDate *time.Time) bool {
	return f.creationDateTo.IsZero() || f.creationDateTo.Equal(*creationDate) || f.creationDateTo.After(*creationDate)
}

func (f *bucketListFilter) filterByNameStringContains(bucketname *string) bool {
	return strings.Contains(*bucketname, *f.bucketNameString)
}
