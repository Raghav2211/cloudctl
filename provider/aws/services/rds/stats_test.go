package rds

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestDBStatisticsFetcher_Fetch_HappyPath(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	client := &fakeCloudWatchClient{
		outputs: map[string]*cloudwatch.GetMetricStatisticsOutput{
			"CPUUtilization":      {Datapoints: []cwtypes.Datapoint{{Average: aws.Float64(20)}, {Average: aws.Float64(40)}}},
			"DatabaseConnections": {Datapoints: []cwtypes.Datapoint{{Average: aws.Float64(5)}}},
			"FreeStorageSpace": {Datapoints: []cwtypes.Datapoint{
				{Average: aws.Float64(2 * 1024 * 1024 * 1024), Timestamp: &older},
				{Average: aws.Float64(1 * 1024 * 1024 * 1024), Timestamp: &newer}, // most recent: must win
			}},
			"ReadLatency":  {Datapoints: []cwtypes.Datapoint{{Average: aws.Float64(0.002)}}},
			"WriteLatency": {Datapoints: []cwtypes.Datapoint{}},
		},
	}
	f := dbStatisticsFetcher{client: client, identifier: "orders-db"}

	stats, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.apiError != nil {
		t.Fatalf("unexpected apiError: %v", stats.apiError)
	}
	if stats.cpuPercent == nil || *stats.cpuPercent != 30 {
		t.Errorf("expected CPU mean 30, got %v", stats.cpuPercent)
	}
	if stats.connectionsAvg == nil || *stats.connectionsAvg != 5 {
		t.Errorf("expected connections avg 5, got %v", stats.connectionsAvg)
	}
	if stats.freeStorageGB == nil || *stats.freeStorageGB != 1 {
		t.Errorf("expected free storage to use the most recent datapoint (1 GB), got %v", stats.freeStorageGB)
	}
	if stats.readLatencyMs == nil || *stats.readLatencyMs != 2 {
		t.Errorf("expected read latency 2ms (0.002s scaled), got %v", stats.readLatencyMs)
	}
	if stats.writeLatencyMs != nil {
		t.Errorf("expected no write latency data, got %v", stats.writeLatencyMs)
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

	dbStatisticsViewer(stats, nil).View() // must not panic
}

func TestDBStatisticsFetcher_Fetch_CPUAPIError(t *testing.T) {
	client := &fakeCloudWatchClient{errs: map[string]error{"CPUUtilization": errors.New("boom")}}
	f := dbStatisticsFetcher{client: client, identifier: "orders-db"}

	stats, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected the fetch itself to succeed with apiError set, got %v", err)
	}
	if stats.apiError == nil {
		t.Fatal("expected apiError to be set when the primary metric call fails")
	}

	dbStatisticsViewer(stats, nil).View() // must not panic
}
