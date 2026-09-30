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
