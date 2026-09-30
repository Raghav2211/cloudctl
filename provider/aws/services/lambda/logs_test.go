package lambda

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// fakeLogsClient implements logsAPI, letting tests substitute it for a real
// *cloudwatchlogs.Client (ADR-0007).
type fakeLogsClient struct {
	out       *cloudwatchlogs.FilterLogEventsOutput
	err       error
	lastInput *cloudwatchlogs.FilterLogEventsInput
}

func (f *fakeLogsClient) FilterLogEvents(_ context.Context, params *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	f.lastInput = params
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestFunctionLogsFetcher_Fetch_HappyPath(t *testing.T) {
	client := &fakeLogsClient{
		out: &cloudwatchlogs.FilterLogEventsOutput{
			Events: []types.FilteredLogEvent{
				{Timestamp: aws.Int64(1700000000000), Message: aws.String("START RequestId: abc\n")},
				{Timestamp: aws.Int64(1700000001000), Message: aws.String("hello world")},
			},
		},
	}
	f := functionLogsFetcher{client: client, functionName: "orders-handler", since: time.Hour, limit: 50}

	logs, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(logs.events))
	}
	if client.lastInput.LogGroupName == nil || *client.lastInput.LogGroupName != "/aws/lambda/orders-handler" {
		t.Errorf("expected log group /aws/lambda/orders-handler, got %v", client.lastInput.LogGroupName)
	}

	functionLogsViewer(logs, nil).View() // must not panic
}

func TestFunctionLogsFetcher_Fetch_FilterPatternPassedThrough(t *testing.T) {
	client := &fakeLogsClient{out: &cloudwatchlogs.FilterLogEventsOutput{}}
	f := functionLogsFetcher{client: client, functionName: "orders-handler", since: time.Hour, limit: 50, filterPattern: "ERROR"}

	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.lastInput.FilterPattern == nil || *client.lastInput.FilterPattern != "ERROR" {
		t.Errorf("expected filter pattern ERROR to be passed through, got %v", client.lastInput.FilterPattern)
	}
}

func TestFunctionLogsFetcher_Fetch_EmptyResult(t *testing.T) {
	client := &fakeLogsClient{out: &cloudwatchlogs.FilterLogEventsOutput{}}
	f := functionLogsFetcher{client: client, functionName: "orders-handler", since: time.Hour, limit: 50}

	logs, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs.events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(logs.events))
	}

	functionLogsViewer(logs, nil).View() // must not panic
}

func TestFunctionLogsFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeLogsClient{err: errors.New("boom")}
	f := functionLogsFetcher{client: client, functionName: "does-not-exist", since: time.Hour, limit: 50}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing FilterLogEvents call, got nil")
	}
}
