package ec2

import (
	ctltime "cloudctl/time"
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// fakeGetMetricStatisticsClient is safe for concurrent use by multiple
// goroutines, matching how *cloudwatch.Client behaves in production; it
// exists to let TestStatisticsFetcher_Fetch_ConcurrentNoRace exercise the
// real statisticsFetcher.Fetch goroutine fan-out under go test -race.
type fakeGetMetricStatisticsClient struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeGetMetricStatisticsClient) GetMetricStatistics(_ context.Context, _ *cloudwatch.GetMetricStatisticsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	return &cloudwatch.GetMetricStatisticsOutput{
		Datapoints: []cwtypes.Datapoint{
			{Minimum: aws.Float64(10), Maximum: aws.Float64(30), Average: aws.Float64(20)},
		},
	}, nil
}

// TestStatisticsFetcher_Fetch_ConcurrentNoRace is a regression test for the
// Track 1A bug where statisticsFetcher.Fetch appended per-instance results
// to a shared slice from unsynchronized goroutines. It exercises the fixed
// errgroup+semaphore/index-write pattern against enough instances that
// `go test -race` would reliably flag the old append-from-goroutines pattern.
func TestStatisticsFetcher_Fetch_ConcurrentNoRace(t *testing.T) {
	const instanceCount = 50

	running := make([]types.Instance, 0, instanceCount)
	for i := 0; i < instanceCount; i++ {
		running = append(running, testInstance(instanceIDFor(i), "running"))
	}

	ec2Client := &fakeDescribeInstancesClient{
		pages: []*ec2.DescribeInstancesOutput{
			{Reservations: []types.Reservation{{Instances: running}}},
		},
	}
	cwClient := &fakeGetMetricStatisticsClient{}

	f := statisticsFetcher{client: ec2Client, cloudwatchClient: cwClient, tz: ctltime.GetTZ("utc")}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.stats) != instanceCount {
		t.Fatalf("expected %d results, got %d", instanceCount, len(out.stats))
	}

	seen := make(map[string]bool, instanceCount)
	for i, stat := range out.stats {
		if stat.instanceId == nil {
			t.Fatalf("result %d has a nil instanceId (index-write landed on the wrong slot)", i)
		}
		if seen[*stat.instanceId] {
			t.Fatalf("instance %s appeared more than once in results", *stat.instanceId)
		}
		seen[*stat.instanceId] = true
		if stat.Average == nil || *stat.Average != 20 {
			t.Fatalf("instance %s: expected Average=20, got %v", *stat.instanceId, stat.Average)
		}
	}
	if cwClient.calls != instanceCount {
		t.Fatalf("expected %d CloudWatch calls, got %d", instanceCount, cwClient.calls)
	}
}

func instanceIDFor(i int) string {
	return "i-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
}
