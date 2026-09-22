package s3

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func testBucket(name string, created time.Time) types.Bucket {
	return types.Bucket{Name: aws.String(name), CreationDate: &created}
}

func TestBucketListFilter_NoOptions_MatchesEverything(t *testing.T) {
	filter := NewBucketListFilter()
	if !filter.applyCustomFilter(testBucket("any-bucket", time.Now())) {
		t.Error("expected no filtering when no options are set")
	}
}

func TestBucketListFilter_ByName(t *testing.T) {
	name := "payments"
	filter := NewBucketListFilter(WithBucketNameFilter(&name))

	if !filter.applyCustomFilter(testBucket("prod-payments-audit-logs", time.Now())) {
		t.Error("expected bucket containing the name substring to match")
	}
	if filter.applyCustomFilter(testBucket("prod-orders-logs", time.Now())) {
		t.Error("expected bucket not containing the name substring to be filtered out")
	}
}

func TestBucketListFilter_ByCreationDate_Wildcard(t *testing.T) {
	date := "2021-09-29T*"
	filter := NewBucketListFilter(WithCreationDateFilter(&date))

	matching := time.Date(2021, 9, 29, 11, 4, 43, 0, time.UTC)
	notMatching := time.Date(2021, 9, 30, 11, 4, 43, 0, time.UTC)

	if !filter.applyCustomFilter(testBucket("b1", matching)) {
		t.Error("expected bucket created on the wildcard-matched day to match")
	}
	if filter.applyCustomFilter(testBucket("b2", notMatching)) {
		t.Error("expected bucket created on a different day to be filtered out")
	}
}

func TestBucketListFilter_ByNameAndCreationDate_BothMustMatch(t *testing.T) {
	name := "payments"
	date := "2021-09-29T*"
	filter := NewBucketListFilter(WithBucketNameFilter(&name), WithCreationDateFilter(&date))

	matchingDate := time.Date(2021, 9, 29, 0, 0, 0, 0, time.UTC)
	if !filter.applyCustomFilter(testBucket("prod-payments", matchingDate)) {
		t.Error("expected bucket matching both name and date to match")
	}
	if filter.applyCustomFilter(testBucket("prod-orders", matchingDate)) {
		t.Error("expected bucket matching only date (not name) to be filtered out")
	}
}
