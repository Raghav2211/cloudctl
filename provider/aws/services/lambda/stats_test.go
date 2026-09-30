package lambda

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// fakeCloudWatchClient implements getMetricStatisticsAPI, returning a
// canned response per MetricName, letting tests substitute it for a real
// *cloudwatch.Client (ADR-0007).
type fakeCloudWatchClient struct {
	outputs map[string]*cloudwatch.GetMetricStatisticsOutput
	errs    map[string]error
}

func (f *fakeCloudWatchClient) GetMetricStatistics(_ context.Context, params *cloudwatch.GetMetricStatisticsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
	name := *params.MetricName
	if err, ok := f.errs[name]; ok {
		return nil, err
	}
	if out, ok := f.outputs[name]; ok {
		return out, nil
	}
	return &cloudwatch.GetMetricStatisticsOutput{}, nil
}

func TestFunctionStatisticsFetcher_Fetch_HappyPath(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	client := &fakeCloudWatchClient{
		outputs: map[string]*cloudwatch.GetMetricStatisticsOutput{
			"Invocations": {Datapoints: []cwtypes.Datapoint{{Sum: aws.Float64(10)}, {Sum: aws.Float64(5)}}},
			"Errors":      {Datapoints: []cwtypes.Datapoint{{Sum: aws.Float64(1)}}},
			"Throttles":   {Datapoints: []cwtypes.Datapoint{}},
			"Duration":    {Datapoints: []cwtypes.Datapoint{{Average: aws.Float64(100)}, {Average: aws.Float64(200)}}},
		},
	}
	f := functionStatisticsFetcher{client: client, functionName: "orders-handler"}

	stats, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.apiError != nil {
		t.Fatalf("unexpected apiError: %v", stats.apiError)
	}
	if stats.invocations == nil || *stats.invocations != 15 {
		t.Errorf("expected invocations sum 15, got %v", stats.invocations)
	}
	if stats.errors == nil || *stats.errors != 1 {
		t.Errorf("expected errors sum 1, got %v", stats.errors)
	}
	if stats.throttles != nil {
		t.Errorf("expected no throttle data, got %v", stats.throttles)
	}
	if stats.durationMs == nil || *stats.durationMs != 150 {
		t.Errorf("expected duration mean 150, got %v", stats.durationMs)
	}
	if stats.aiSummary != "" {
		t.Errorf("expected no AI summary with Ollama unreachable, got %q", stats.aiSummary)
	}
	if stats.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to explain why the summary is missing")
	}
	if stats.aiRecommendationsUnavailable == "" {
		t.Error("expected aiRecommendationsUnavailable to explain why recommendations are missing")
	}

	functionStatisticsViewer(stats, nil).View() // must not panic
}

func TestFunctionStatisticsFetcher_Fetch_InvocationsAPIError(t *testing.T) {
	client := &fakeCloudWatchClient{errs: map[string]error{"Invocations": errors.New("boom")}}
	f := functionStatisticsFetcher{client: client, functionName: "orders-handler"}

	stats, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected the fetch itself to succeed with apiError set, got %v", err)
	}
	if stats.apiError == nil {
		t.Fatal("expected apiError to be set when the primary metric call fails")
	}

	functionStatisticsViewer(stats, nil).View() // must not panic
}

func TestFunctionStatisticsFetcher_Fetch_SecondaryMetricErrorDegradesGracefully(t *testing.T) {
	client := &fakeCloudWatchClient{
		outputs: map[string]*cloudwatch.GetMetricStatisticsOutput{
			"Invocations": {Datapoints: []cwtypes.Datapoint{{Sum: aws.Float64(10)}}},
		},
		errs: map[string]error{"Errors": errors.New("boom")},
	}
	f := functionStatisticsFetcher{client: client, functionName: "orders-handler"}

	stats, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.invocations == nil || *stats.invocations != 10 {
		t.Errorf("expected invocations to still be populated, got %v", stats.invocations)
	}
	if stats.errors != nil {
		t.Errorf("expected errors metric to degrade to nil on failure, got %v", stats.errors)
	}
}
