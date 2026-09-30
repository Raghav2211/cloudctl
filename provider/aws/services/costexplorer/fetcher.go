// Package costexplorer fetches real AWS spend via ce:GetCostAndUsage — the
// primary data source for `ctl aws cost` (see cloudctl/cost's package doc
// for the Cost-Explorer-then-idle-scan fallback design, ADR-0023).
package costexplorer

import (
	"cloudctl/cost"
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/smithy-go"
)

// getCostAndUsageAPI is the minimal client capability this package needs,
// letting tests substitute a fake instead of a real *costexplorer.Client
// (ADR-0007).
type getCostAndUsageAPI interface {
	GetCostAndUsage(ctx context.Context, params *costexplorer.GetCostAndUsageInput, optFns ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

// SummaryFetcher fetches real AWS spend by service over the last Days days
// — the same data AWS itself bills from.
type SummaryFetcher struct {
	client getCostAndUsageAPI
	days   int32
}

func NewSummaryFetcher(cfg aws.Config, days int32) *SummaryFetcher {
	return &SummaryFetcher{client: costexplorer.NewFromConfig(cfg), days: days}
}

const costMetric = "UnblendedCost"

func (f *SummaryFetcher) Fetch(ctx context.Context) (*cost.CostSummary, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -int(f.days))
	startStr, endStr := start.Format("2006-01-02"), end.Format("2006-01-02")

	out, err := f.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(startStr), End: aws.String(endStr)},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{costMetric},
		GroupBy:     []types.GroupDefinition{{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")}},
	})
	if err != nil {
		return nil, err
	}

	// Granularity=MONTHLY can return more than one bucket if the window
	// spans a calendar-month boundary — sum across every bucket so the
	// summary is one number per service for the whole window, not a
	// per-month breakdown the caller would have to add up itself.
	totals := map[string]float64{}
	unit := "USD"
	for _, period := range out.ResultsByTime {
		for _, group := range period.Groups {
			if len(group.Keys) == 0 {
				continue
			}
			service := group.Keys[0]
			metric, ok := group.Metrics[costMetric]
			if !ok || metric.Amount == nil {
				continue
			}
			amount, parseErr := strconv.ParseFloat(*metric.Amount, 64)
			if parseErr != nil {
				continue
			}
			totals[service] += amount
			if metric.Unit != nil && *metric.Unit != "" {
				unit = *metric.Unit
			}
		}
	}

	byService := make([]cost.ServiceCost, 0, len(totals))
	var total float64
	for service, amount := range totals {
		byService = append(byService, cost.ServiceCost{Service: service, Amount: amount, Unit: unit})
		total += amount
	}
	sort.Slice(byService, func(i, j int) bool { return byService[i].Amount > byService[j].Amount })

	return &cost.CostSummary{
		PeriodStart: startStr,
		PeriodEnd:   endStr,
		TotalAmount: total,
		Unit:        unit,
		ByService:   byService,
	}, nil
}

// IsAccessDenied reports whether err is Cost Explorer rejecting the call
// for lack of ce:GetCostAndUsage permission — the trigger for `ctl aws
// cost`'s idle-scan fallback.
func IsAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDeniedException", "AccessDenied", "UnauthorizedOperation":
			return true
		}
	}
	return false
}
