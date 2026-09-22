package s3

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeListObjectsClient implements listObjectsAPI, returning one page from
// pages per call, in order.
type fakeListObjectsClient struct {
	pages []*s3.ListObjectsOutput
	err   error
	calls int
}

func (f *fakeListObjectsClient) ListObjects(_ context.Context, _ *s3.ListObjectsInput, _ ...func(*s3.Options)) (*s3.ListObjectsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.calls >= len(f.pages) {
		return &s3.ListObjectsOutput{}, nil
	}
	page := f.pages[f.calls]
	f.calls++
	return page, nil
}

func testObject(key string) types.Object {
	return types.Object{Key: aws.String(key)}
}

func TestFetchBucketObjects_SinglePage(t *testing.T) {
	client := &fakeListObjectsClient{
		pages: []*s3.ListObjectsOutput{
			{Contents: []types.Object{testObject("a"), testObject("b")}, IsTruncated: aws.Bool(false)},
		},
	}

	objects, notice, err := fetchBucketObjects(context.Background(), "bucket", nil, 1000, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notice != nil {
		t.Fatalf("unexpected notice: %v", notice)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
}

// Regression test for the Track 1B fix: fetch's accumulator used to be
// passed by value and reassigned locally, so the caller's slice was never
// actually updated across recursive pagination calls — ctl s3 list-objects
// always returned "no object found" regardless of bucket contents. This
// exercises exactly that multi-page path.
func TestFetchBucketObjects_MultiPageAccumulatesAcrossRecursion(t *testing.T) {
	client := &fakeListObjectsClient{
		pages: []*s3.ListObjectsOutput{
			{Contents: []types.Object{testObject("a"), testObject("b")}, IsTruncated: aws.Bool(true)},
			{Contents: []types.Object{testObject("c")}, IsTruncated: aws.Bool(false)},
		},
	}

	objects, notice, err := fetchBucketObjects(context.Background(), "bucket", nil, 1000, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notice != nil {
		t.Fatalf("unexpected notice: %v", notice)
	}
	if len(objects) != 3 {
		t.Fatalf("expected all 3 objects accumulated across 2 pages, got %d: %+v", len(objects), objects)
	}
	if client.calls != 2 {
		t.Fatalf("expected the paginator to stop after 2 calls, made %d", client.calls)
	}
}

func TestFetchBucketObjects_MaxKeysReached_ReturnsNotice(t *testing.T) {
	client := &fakeListObjectsClient{
		pages: []*s3.ListObjectsOutput{
			{Contents: []types.Object{testObject("a"), testObject("b")}, IsTruncated: aws.Bool(true)},
		},
	}

	objects, notice, err := fetchBucketObjects(context.Background(), "bucket", nil, 2, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects (up to max-keys), got %d", len(objects))
	}
	if notice == nil {
		t.Fatal("expected a non-fatal notice when the bucket has more objects than max-keys")
	}
}

func TestFetchBucketObjects_EmptyResult_ReturnsError(t *testing.T) {
	client := &fakeListObjectsClient{
		pages: []*s3.ListObjectsOutput{{Contents: []types.Object{}, IsTruncated: aws.Bool(false)}},
	}

	objects, notice, err := fetchBucketObjects(context.Background(), "bucket", nil, 1000, client)
	if err == nil {
		t.Fatal("expected an error when no objects are found, got nil")
	}
	if objects != nil || notice != nil {
		t.Fatalf("expected nil objects/notice alongside the error, got objects=%+v notice=%v", objects, notice)
	}
}

func TestFetchBucketObjects_APIError(t *testing.T) {
	client := &fakeListObjectsClient{err: errors.New("boom")}

	if _, _, err := fetchBucketObjects(context.Background(), "bucket", nil, 1000, client); err == nil {
		t.Fatal("expected an error from a failing ListObjects call, got nil")
	}
}
