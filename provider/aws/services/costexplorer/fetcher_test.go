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
	out *costexplorer.GetCostAndUsageOutput
	err error
}

func (f *fakeCostExplorerClient) GetCostAndUsage(_ context.Context, _ *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	if f.err != nil {
		return nil, f.err
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
