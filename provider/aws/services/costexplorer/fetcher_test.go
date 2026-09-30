package costexplorer

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/smithy-go"
)

type fakeCostExplorerClient struct {
	out   *costexplorer.GetCostAndUsageOutput   // single-page fixture
	pages []*costexplorer.GetCostAndUsageOutput // multi-page fixture, returned in order across successive calls
	err   error

	callCount int
	gotInputs []*costexplorer.GetCostAndUsageInput
}

func (f *fakeCostExplorerClient) GetCostAndUsage(_ context.Context, in *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	f.gotInputs = append(f.gotInputs, in)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.pages) > 0 {
		out := f.pages[f.callCount]
		f.callCount++
		return out, nil
	}
	return f.out, nil
}

func TestSummaryFetcher_Fetch_SumsAcrossPeriodsAndSortsDescending(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
					{Keys: []string{"Amazon S3"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("10.00"), Unit: aws.String("USD")}}},
				}},
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("25.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalAmount != 85.0 {
		t.Errorf("expected total 85.0, got %v", summary.TotalAmount)
	}
	if len(summary.ByService) != 2 || summary.ByService[0].Service != "Amazon EC2" || summary.ByService[0].Amount != 75.0 {
		t.Fatalf("expected EC2 (summed to 75.0) first, got %+v", summary.ByService)
	}
	if summary.ByService[1].Service != "Amazon S3" || summary.ByService[1].Amount != 10.0 {
		t.Fatalf("expected S3 second at 10.0, got %+v", summary.ByService)
	}
}

func TestSummaryFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeCostExplorerClient{err: errors.New("boom")}
	f := &SummaryFetcher{client: client, days: 30}

	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("expected an error from a failing GetCostAndUsage call, got nil")
	}
}

type fakeAPIError struct{ code string }

func (e fakeAPIError) Error() string     { return e.code }
func (e fakeAPIError) ErrorCode() string { return e.code }
func (e fakeAPIError) ErrorMessage() string {
	return "denied"
}
func (e fakeAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestIsAccessDenied(t *testing.T) {
	if !IsAccessDenied(fakeAPIError{code: "AccessDeniedException"}) {
		t.Error("expected AccessDeniedException to be classified as access denied")
	}
	if IsAccessDenied(errors.New("some other error")) {
		t.Error("expected a plain error to not be classified as access denied")
	}
	if IsAccessDenied(fakeAPIError{code: "ThrottlingException"}) {
		t.Error("expected a throttling error to not be classified as access denied")
	}
}

func TestSummaryFetcher_Fetch_NoResourceTagKeyLeavesByResourceEmpty(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30} // resourceTagKey left as the zero value

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 0 {
		t.Fatalf("expected no per-resource breakdown when resourceTagKey is unset, got %+v", summary.ByResource)
	}
}

func TestSummaryFetcher_Fetch_GroupsByResourceTag(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$i-0abc123"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("30.00"), Unit: aws.String("USD")}}},
					{Keys: []string{"Amazon EC2", "Name$i-0def456"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("20.00"), Unit: aws.String("USD")}}},
				}},
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$i-0abc123"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("10.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 2 {
		t.Fatalf("expected 2 resources, got %+v", summary.ByResource)
	}
	if summary.ByResource[0].ResourceID != "i-0abc123" || summary.ByResource[0].Amount != 40.0 {
		t.Errorf("expected i-0abc123 summed across periods to 40.0 first, got %+v", summary.ByResource[0])
	}
	if summary.ByResource[1].ResourceID != "i-0def456" || summary.ByResource[1].Amount != 20.0 {
		t.Errorf("expected i-0def456 at 20.0 second, got %+v", summary.ByResource[1])
	}
	if summary.ByResource[0].Service != "Amazon EC2" {
		t.Errorf("expected resource cost tagged with its service, got %+v", summary.ByResource[0])
	}
	if len(summary.ByService) != 1 || summary.ByService[0].Amount != 60.0 {
		t.Fatalf("expected service total 60.0 unaffected by resource grouping, got %+v", summary.ByService)
	}
}

func TestSummaryFetcher_Fetch_UntaggedResourcesBucketAsUntagged(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("15.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 1 || summary.ByResource[0].ResourceID != "untagged" {
		t.Fatalf("expected untagged spend to bucket as \"untagged\", got %+v", summary.ByResource)
	}
}

func TestSummaryFetcher_Fetch_GroupMissingResourceKeyStillCountsServiceTotal(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 0 {
		t.Fatalf("expected no resource entries when a group is missing the resource key, got %+v", summary.ByResource)
	}
	if len(summary.ByService) != 1 || summary.ByService[0].Amount != 50.0 {
		t.Fatalf("expected service total to still be counted even without a resource key, got %+v", summary.ByService)
	}
}

func TestSummaryFetcher_Fetch_PaginatesAcrossNextPageToken(t *testing.T) {
	client := &fakeCostExplorerClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			{
				NextPageToken: aws.String("page-2-token"),
				ResultsByTime: []types.ResultByTime{
					{Groups: []types.Group{
						{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
					}},
				},
			},
			{
				ResultsByTime: []types.ResultByTime{
					{Groups: []types.Group{
						{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("25.00"), Unit: aws.String("USD")}}},
					}},
				},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalAmount != 75.0 {
		t.Fatalf("expected total 75.0 summed across both pages, got %v", summary.TotalAmount)
	}
	if len(summary.ByService) != 1 || summary.ByService[0].Amount != 75.0 {
		t.Fatalf("expected EC2 summed to 75.0 across pages, got %+v", summary.ByService)
	}
	if len(client.gotInputs) != 2 {
		t.Fatalf("expected exactly 2 requests (one per page), got %d", len(client.gotInputs))
	}
	if client.gotInputs[1].NextPageToken == nil || *client.gotInputs[1].NextPageToken != "page-2-token" {
		t.Fatalf("expected the second request to carry the first response's NextPageToken, got %+v", client.gotInputs[1].NextPageToken)
	}
}

func TestSummaryFetcher_Fetch_NoResourceTagKeyRequestsServiceOnlyGroupBy(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30}

	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.gotInputs) != 1 {
		t.Fatalf("expected exactly one request, got %d", len(client.gotInputs))
	}
	groupBy := client.gotInputs[0].GroupBy
	if len(groupBy) != 1 || groupBy[0].Type != types.GroupDefinitionTypeDimension || *groupBy[0].Key != "SERVICE" {
		t.Fatalf("expected GroupBy=[{DIMENSION SERVICE}] when resourceTagKey is unset, got %+v", groupBy)
	}
}

func TestSummaryFetcher_Fetch_ResourceTagKeyAddsTagGroupBy(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$i-1"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	groupBy := client.gotInputs[0].GroupBy
	if len(groupBy) != 2 || groupBy[1].Type != types.GroupDefinitionTypeTag || *groupBy[1].Key != "Name" {
		t.Fatalf("expected GroupBy to include TAG:Name when resourceTagKey is set, got %+v", groupBy)
	}
}

// TestSummaryFetcher_Fetch_TiedAmountsSortDeterministically guards against
// map-iteration-order flakiness: resourceTotals/serviceTotals are Go maps,
// and sort.Slice is not stable, so without an explicit tie-break, resources
// with equal amounts (common — many near-zero or identically-priced
// resources) would render in a different order on every run, breaking any
// diff/script relying on --output json and undermining Track B's
// period-over-period comparison.
func TestSummaryFetcher_Fetch_TiedAmountsSortDeterministically(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$i-c"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("10.00"), Unit: aws.String("USD")}}},
					{Keys: []string{"Amazon EC2", "Name$i-a"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("10.00"), Unit: aws.String("USD")}}},
					{Keys: []string{"Amazon EC2", "Name$i-b"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("10.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}

	for i := 0; i < 10; i++ {
		f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}
		summary, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		if len(summary.ByResource) != 3 {
			t.Fatalf("run %d: expected 3 resources, got %+v", i, summary.ByResource)
		}
		got := []string{summary.ByResource[0].ResourceID, summary.ByResource[1].ResourceID, summary.ByResource[2].ResourceID}
		want := []string{"i-a", "i-b", "i-c"}
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("run %d: expected deterministic tie-broken order %v, got %v", i, want, got)
			}
		}
	}
}

func TestParseTagValue(t *testing.T) {
	cases := map[string]string{
		"Name$i-0abc123":  "i-0abc123",
		"Name$":           "untagged",
		"Name":            "Name",
		"Name$multi$part": "multi$part",
	}
	for input, want := range cases {
		if got := parseTagValue(input); got != want {
			t.Errorf("parseTagValue(%q) = %q, want %q", input, got, want)
		}
	}
}
