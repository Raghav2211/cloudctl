package changes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

// fakeCloudTrailClient implements cloudTrailAPI with canned, per-call
// responses, letting tests substitute it for a real *cloudtrail.Client
// (ADR-0007).
type fakeCloudTrailClient struct {
	pages      []*cloudtrail.LookupEventsOutput
	calls      int
	err        error
	lastInputs []*cloudtrail.LookupEventsInput
}

func (f *fakeCloudTrailClient) LookupEvents(_ context.Context, params *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	f.lastInputs = append(f.lastInputs, params)
	if f.err != nil {
		return nil, f.err
	}
	if f.calls >= len(f.pages) {
		return &cloudtrail.LookupEventsOutput{}, nil
	}
	page := f.pages[f.calls]
	f.calls++
	return page, nil
}

func testEvent(name string) types.Event {
	now := time.Now()
	return types.Event{
		EventId:     aws.String("evt-" + name),
		EventName:   aws.String(name),
		EventTime:   &now,
		EventSource: aws.String("ec2.amazonaws.com"),
		Username:    aws.String("alice"),
		Resources:   []types.Resource{{ResourceName: aws.String("i-abc123"), ResourceType: aws.String("Instance")}},
	}
}

func TestChangeListFetcher_Fetch_HappyPath(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	client := &fakeCloudTrailClient{
		pages: []*cloudtrail.LookupEventsOutput{
			{Events: []types.Event{testEvent("RunInstances"), testEvent("AuthorizeSecurityGroupIngress")}},
		},
	}
	f := changeListFetcher{client: client, since: 2 * time.Hour, limit: 50}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(out.changes))
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

	changeListViewer(out, nil).View() // must not panic
}

func TestChangeListEvidence_CapsAtMaxChangesForNarration(t *testing.T) {
	changes := make([]*change, 0, maxChangesForNarration+5)
	for i := 0; i < maxChangesForNarration+5; i++ {
		c := testEvent("RunInstances")
		changes = append(changes, newChange(c))
	}
	data := &changeListOutput{changes: changes}

	facts := changeListEvidence(data)
	if len(facts) != maxChangesForNarration {
		t.Fatalf("expected evidence capped at %d, got %d", maxChangesForNarration, len(facts))
	}
}

func TestChangeListFetcher_Fetch_FiltersByResource(t *testing.T) {
	client := &fakeCloudTrailClient{pages: []*cloudtrail.LookupEventsOutput{{Events: []types.Event{testEvent("RunInstances")}}}}
	f := changeListFetcher{client: client, resourceName: "i-abc123", since: time.Hour, limit: 50}

	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	input := client.lastInputs[0]
	if len(input.LookupAttributes) != 1 || *input.LookupAttributes[0].AttributeValue != "i-abc123" {
		t.Errorf("expected a ResourceName lookup attribute for i-abc123, got %+v", input.LookupAttributes)
	}
}

func TestChangeListFetcher_Fetch_PaginatesUpToLimit(t *testing.T) {
	nextToken := "page2"
	client := &fakeCloudTrailClient{
		pages: []*cloudtrail.LookupEventsOutput{
			{Events: []types.Event{testEvent("A"), testEvent("B")}, NextToken: &nextToken},
			{Events: []types.Event{testEvent("C"), testEvent("D")}},
		},
	}
	f := changeListFetcher{client: client, since: time.Hour, limit: 3}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.changes) != 3 {
		t.Fatalf("expected exactly 3 changes (limit), got %d", len(out.changes))
	}
	if client.calls != 2 {
		t.Fatalf("expected 2 pages fetched to reach the limit, got %d", client.calls)
	}
}

func TestChangeListFetcher_Fetch_FiltersByEventName_ServerSide(t *testing.T) {
	client := &fakeCloudTrailClient{pages: []*cloudtrail.LookupEventsOutput{{Events: []types.Event{testEvent("RunInstances")}}}}
	f := changeListFetcher{client: client, eventName: "RunInstances", since: time.Hour, limit: 50}

	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	input := client.lastInputs[0]
	if len(input.LookupAttributes) != 1 ||
		input.LookupAttributes[0].AttributeKey != "EventName" ||
		*input.LookupAttributes[0].AttributeValue != "RunInstances" {
		t.Errorf("expected an EventName lookup attribute for RunInstances, got %+v", input.LookupAttributes)
	}
}

func TestChangeListFetcher_Fetch_ResourceAndEventName_EventNameAppliedClientSide(t *testing.T) {
	client := &fakeCloudTrailClient{pages: []*cloudtrail.LookupEventsOutput{
		{Events: []types.Event{testEvent("RunInstances"), testEvent("StopInstances")}},
	}}
	f := changeListFetcher{client: client, resourceName: "i-abc123", eventName: "RunInstances", since: time.Hour, limit: 50}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The server-side slot went to ResourceName (resource takes priority),
	// so both events came back from the fake client — EventName must have
	// been applied client-side to drop StopInstances.
	input := client.lastInputs[0]
	if input.LookupAttributes[0].AttributeKey != "ResourceName" {
		t.Errorf("expected ResourceName to win the server-side slot, got %+v", input.LookupAttributes)
	}
	if len(out.changes) != 1 || *out.changes[0].eventName != "RunInstances" {
		t.Fatalf("expected only RunInstances to survive the client-side event-name filter, got %+v", out.changes)
	}
}

func TestChangeListFetcher_Fetch_ExcludesEvents(t *testing.T) {
	client := &fakeCloudTrailClient{pages: []*cloudtrail.LookupEventsOutput{
		{Events: []types.Event{testEvent("AssumeRole"), testEvent("RunInstances"), testEvent("ConsoleLogin")}},
	}}
	f := changeListFetcher{
		client:        client,
		excludeEvents: map[string]struct{}{"AssumeRole": {}, "ConsoleLogin": {}},
		since:         time.Hour,
		limit:         50,
	}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.changes) != 1 || *out.changes[0].eventName != "RunInstances" {
		t.Fatalf("expected only RunInstances to survive exclusion, got %+v", out.changes)
	}
}

func TestChangeListFetcher_Fetch_ExclusionPaginatesPastFilteredPages(t *testing.T) {
	nextToken := "page2"
	client := &fakeCloudTrailClient{pages: []*cloudtrail.LookupEventsOutput{
		// entirely excluded page — must not be mistaken for "done"
		{Events: []types.Event{testEvent("AssumeRole"), testEvent("AssumeRole")}, NextToken: &nextToken},
		{Events: []types.Event{testEvent("RunInstances")}},
	}}
	f := changeListFetcher{
		client:        client,
		excludeEvents: map[string]struct{}{"AssumeRole": {}},
		since:         time.Hour,
		limit:         50,
	}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.changes) != 1 {
		t.Fatalf("expected 1 surviving change after paginating past a fully-excluded page, got %d", len(out.changes))
	}
	if client.calls != 2 {
		t.Fatalf("expected 2 pages fetched, got %d", client.calls)
	}
}

func TestChangeListFetcher_Fetch_EmptyResult(t *testing.T) {
	client := &fakeCloudTrailClient{pages: []*cloudtrail.LookupEventsOutput{{}}}
	f := changeListFetcher{client: client, since: time.Hour, limit: 50}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no changes are found, got nil")
	}
}

func TestChangeListFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeCloudTrailClient{err: errors.New("boom")}
	f := changeListFetcher{client: client, since: time.Hour, limit: 50}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing LookupEvents call, got nil")
	}
}
