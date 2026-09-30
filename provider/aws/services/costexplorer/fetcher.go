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
	"strings"
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

// SummaryFetcher fetches real AWS spend by service (and, if resourceTagKey
// is set, by that cost-allocation tag) over the last Days days — the same
// data AWS itself bills from.
type SummaryFetcher struct {
	client         getCostAndUsageAPI
	days           int32
	resourceTagKey string
}

func NewSummaryFetcher(cfg aws.Config, days int32, resourceTagKey string) *SummaryFetcher {
	return &SummaryFetcher{client: costexplorer.NewFromConfig(cfg), days: days, resourceTagKey: resourceTagKey}
}

const costMetric = "UnblendedCost"

func (f *SummaryFetcher) Fetch(ctx context.Context) (*cost.CostSummary, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -int(f.days))
	startStr, endStr := start.Format("2006-01-02"), end.Format("2006-01-02")

	groupBy := []types.GroupDefinition{
		{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
	}
	if f.resourceTagKey != "" {
		groupBy = append(groupBy, types.GroupDefinition{Type: types.GroupDefinitionTypeTag, Key: aws.String(f.resourceTagKey)})
	}

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(startStr), End: aws.String(endStr)},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{costMetric},
		GroupBy:     groupBy,
	}

	// Granularity=MONTHLY can return more than one bucket if the window
	// spans a calendar-month boundary — sum across every bucket so the
	// summary is one number per service (and, if requested, per resource)
	// for the whole window, not a per-month breakdown the caller would
	// have to add up itself.
	serviceTotals := map[string]float64{}
	resourceTotals := map[[2]string]float64{} // [service, resourceID] -> amount
	unit := "USD"

	// Adding the TAG dimension multiplies group cardinality (services ×
	// distinct tag values), which can easily exceed one page on a
	// real account — loop on NextPageToken until Cost Explorer stops
	// returning one, or every total silently reflects only the first page.
	for {
		out, err := f.client.GetCostAndUsage(ctx, input)
		if err != nil {
			return nil, err
		}

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
				serviceTotals[service] += amount
				if metric.Unit != nil && *metric.Unit != "" {
					unit = *metric.Unit
				}

				if f.resourceTagKey != "" && len(group.Keys) >= 2 {
					resourceID := parseTagValue(group.Keys[1])
					resourceTotals[[2]string{service, resourceID}] += amount
				}
			}
		}

		if out.NextPageToken == nil || *out.NextPageToken == "" {
			break
		}
		input.NextPageToken = out.NextPageToken
	}

	byService := make([]cost.ServiceCost, 0, len(serviceTotals))
	var total float64
	for service, amount := range serviceTotals {
		byService = append(byService, cost.ServiceCost{Service: service, Amount: amount, Unit: unit})
		total += amount
	}
	sort.Slice(byService, func(i, j int) bool {
		if byService[i].Amount != byService[j].Amount {
			return byService[i].Amount > byService[j].Amount
		}
		return byService[i].Service < byService[j].Service
	})

	byResource := make([]cost.ResourceCost, 0, len(resourceTotals))
	for key, amount := range resourceTotals {
		byResource = append(byResource, cost.ResourceCost{Service: key[0], ResourceID: key[1], Amount: amount, Unit: unit})
	}
	sort.Slice(byResource, func(i, j int) bool {
		if byResource[i].Amount != byResource[j].Amount {
			return byResource[i].Amount > byResource[j].Amount
		}
		if byResource[i].Service != byResource[j].Service {
			return byResource[i].Service < byResource[j].Service
		}
		return byResource[i].ResourceID < byResource[j].ResourceID
	})

	return &cost.CostSummary{
		PeriodStart: startStr,
		PeriodEnd:   endStr,
		TotalAmount: total,
		Unit:        unit,
		ByService:   byService,
		ByResource:  byResource,
	}, nil
}

// parseTagValue extracts a tag's value from Cost Explorer's tag-group key
// format ("<TagKey>$<TagValue>"), returning "untagged" when the value part
// is empty — spend Cost Explorer could attribute to the service but not to
// any specific tagged resource. A key with no "$" at all (not expected from
// a real Cost Explorer response) falls back to returning it unchanged
// rather than panicking.
func parseTagValue(rawKey string) string {
	value := rawKey
	if parts := strings.SplitN(rawKey, "$", 2); len(parts) == 2 {
		value = parts[1]
	}
	if value == "" {
		return "untagged"
	}
	return value
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
