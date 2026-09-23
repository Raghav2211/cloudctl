package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeGetObjectClient implements manager.DownloadAPIClient, tracking the
// maximum number of GetObject calls in flight at once, so the test can
// assert concurrency actually stayed bounded — not just that all objects
// were eventually downloaded.
type fakeGetObjectClient struct {
	inFlight    int64
	maxInFlight int64
}

func (f *fakeGetObjectClient) GetObject(ctx context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	current := atomic.AddInt64(&f.inFlight, 1)
	defer atomic.AddInt64(&f.inFlight, -1)

	for {
		max := atomic.LoadInt64(&f.maxInFlight)
		if current <= max || atomic.CompareAndSwapInt64(&f.maxInFlight, max, current) {
			break
		}
	}

	body := []byte(fmt.Sprintf("content of %s", *params.Key))
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: aws.Int64(int64(len(body))),
	}, nil
}

// TestBucketObjectsDownloadFetcher_Fetch_Recursive_BoundsConcurrency is the
// ADR-0018 regression test: before this fix, the recursive download path
// launched one unbounded goroutine per object key. This confirms every
// object is still downloaded and collected (no data loss — the old
// implementation's wg.Wait()-before-deadline ordering bug made the 30s
// collection deadline meaningless, but this test would also have caught
// outright missing results), and that concurrent GetObject calls never
// exceed maxConcurrentObjectDownloads.
func TestBucketObjectsDownloadFetcher_Fetch_Recursive_BoundsConcurrency(t *testing.T) {
	const objectCount = 50
	contents := make([]types.Object, objectCount)
	for i := range contents {
		contents[i] = testObject(fmt.Sprintf("prefix/object-%d", i))
	}

	getClient := &fakeGetObjectClient{}
	f := bucketObjectsDownloadFetcher{
		client:     &fakeListObjectsClient{pages: []*s3.ListObjectsOutput{{Contents: contents}}},
		downloader: manager.NewDownloader(getClient),
		bucketName: "my-bucket",
		key:        "prefix/",
		path:       t.TempDir(),
		recursive:  true,
	}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.objectsDownloadSummary) != objectCount {
		t.Fatalf("expected %d results, got %d (results were lost)", objectCount, len(summary.objectsDownloadSummary))
	}
	for _, s := range summary.objectsDownloadSummary {
		if s == nil {
			t.Fatal("expected every slice slot to be populated, got a nil summary")
		}
		if s.err != nil {
			t.Errorf("unexpected per-object error: %v", s.err)
		}
	}

	maxInFlight := atomic.LoadInt64(&getClient.maxInFlight)
	t.Logf("peak concurrent GetObject calls observed: %d (bound: %d)", maxInFlight, maxConcurrentObjectDownloads)
	if maxInFlight > maxConcurrentObjectDownloads {
		t.Errorf("expected concurrent GetObject calls to stay <= %d, observed a peak of %d",
			maxConcurrentObjectDownloads, maxInFlight)
	}
	if maxInFlight == 0 {
		t.Error("expected at least one GetObject call to have been observed")
	}
}

func TestBucketObjectsDownloadFetcher_Fetch_Recursive_EmptyPrefix_ReturnsWarn(t *testing.T) {
	f := bucketObjectsDownloadFetcher{
		client:     &fakeListObjectsClient{pages: []*s3.ListObjectsOutput{{}}},
		downloader: manager.NewDownloader(&fakeGetObjectClient{}),
		bucketName: "my-bucket",
		key:        "nothing-here/",
		path:       t.TempDir(),
		recursive:  true,
	}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no objects match the given prefix")
	}
}
