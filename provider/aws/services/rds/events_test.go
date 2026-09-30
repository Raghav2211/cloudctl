package rds

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// fakeEventsClient implements eventsAPI with a canned response, letting
// tests substitute it for a real *rds.Client (ADR-0007).
type fakeEventsClient struct {
	out       *rds.DescribeEventsOutput
	err       error
	lastInput *rds.DescribeEventsInput
}

func (f *fakeEventsClient) DescribeEvents(_ context.Context, params *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	f.lastInput = params
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestDBEventListFetcher_Fetch_HappyPath(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	now := time.Now()
	client := &fakeEventsClient{
		out: &rds.DescribeEventsOutput{Events: []types.Event{
			{Date: &now, Message: aws.String("Backup started"), EventCategories: []string{"backup"}},
			{Date: &now, Message: aws.String("Backup finished"), EventCategories: []string{"backup"}},
		}},
	}
	f := dbEventListFetcher{client: client, identifier: "orders-db", since: 24 * time.Hour}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(out.events))
	}
	if client.lastInput.SourceIdentifier == nil || *client.lastInput.SourceIdentifier != "orders-db" {
		t.Errorf("expected SourceIdentifier orders-db, got %v", client.lastInput.SourceIdentifier)
	}
	if client.lastInput.Duration == nil || *client.lastInput.Duration != 1440 {
		t.Errorf("expected Duration 1440 minutes (24h), got %v", client.lastInput.Duration)
	}
	if out.aiSummary != "" {
		t.Errorf("expected no AI summary with Ollama unreachable, got %q", out.aiSummary)
	}
	if out.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to explain why the summary is missing")
	}
	if out.aiRecommendationsUnavailable == "" {
		t.Error("expected aiRecommendationsUnavailable to explain why recommendations are missing")
	}

	dbEventListViewer(out, nil).View() // must not panic
}

func TestDBEventListEvidence_CapsAtMaxEventsForNarration(t *testing.T) {
	now := time.Now()
	events := make([]*dbEvent, 0, maxEventsForNarration+5)
	for i := 0; i < maxEventsForNarration+5; i++ {
		msg := "event"
		events = append(events, &dbEvent{date: &now, message: &msg})
	}
	data := &dbEventListOutput{identifier: "orders-db", events: events}

	facts := dbEventListEvidence(data)
	if len(facts) != maxEventsForNarration {
		t.Fatalf("expected evidence capped at %d, got %d", maxEventsForNarration, len(facts))
	}
}

func TestDBEventListFetcher_Fetch_EmptyResult(t *testing.T) {
	client := &fakeEventsClient{out: &rds.DescribeEventsOutput{}}
	f := dbEventListFetcher{client: client, identifier: "orders-db", since: time.Hour}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no events are found, got nil")
	}
}

func TestDBEventListFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeEventsClient{err: errors.New("boom")}
	f := dbEventListFetcher{client: client, identifier: "orders-db", since: time.Hour}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeEvents call, got nil")
	}
}
