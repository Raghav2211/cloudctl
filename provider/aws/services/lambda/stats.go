package lambda

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// getMetricStatisticsAPI is the minimal client capability this file needs,
// letting tests substitute a fake instead of a real *cloudwatch.Client.
type getMetricStatisticsAPI interface {
	GetMetricStatistics(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error)
}

// functionStatistics is `ctl aws lambda stats <name>`'s output: the four
// CloudWatch metrics that matter most for a Lambda function's operational
// health, over the last 24 hours.
type functionStatistics struct {
	functionName string

	invocations *float64 // sum
	errors      *float64 // sum
	throttles   *float64 // sum
	durationMs  *float64 // mean of hourly averages — an approximation, not a true 24h average

	apiError *ctlaws.ErrorInfo

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}

type functionStatisticsFetcher struct {
	client       getMetricStatisticsAPI
	functionName string
}

// lambdaMetricWindow is how far back functionStatisticsFetcher looks, and
// the period each datapoint covers — a 24h/1h window keeps this to a single
// page of datapoints per metric while still giving a same-day operational
// picture.
const lambdaMetricWindow = 24 * time.Hour
const lambdaMetricPeriodSeconds = 3600

func (f functionStatisticsFetcher) Fetch(ctx context.Context) (*functionStatistics, error) {
	end := time.Now()
	start := end.Add(-lambdaMetricWindow)

	stats := &functionStatistics{functionName: f.functionName}

	viewer.SetProgress(ctx, fmt.Sprintf("Fetching CloudWatch statistics for %s...", f.functionName))
	sum, err := f.sumMetric(ctx, "Invocations", start, end)
	if err != nil {
		stats.apiError = ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		return stats, nil
	}
	stats.invocations = sum

	if sum, err := f.sumMetric(ctx, "Errors", start, end); err == nil {
		stats.errors = sum
	}
	if sum, err := f.sumMetric(ctx, "Throttles", start, end); err == nil {
		stats.throttles = sum
	}
	if avg, err := f.averageMetric(ctx, "Duration", start, end); err == nil {
		stats.durationMs = avg
	}

	stats.applyAINarration(ctx, ai.NewClientFromEnv(), functionStatisticsEvidence(stats))

	return stats, nil
}

// functionStatisticsEvidence converts already-fetched metrics into
// Fact-tagged evidence for Summarize/Recommend — only metrics that actually
// returned data contribute a fact, mirroring every other evidence builder
// in this codebase's partial-failure tolerance.
func functionStatisticsEvidence(stats *functionStatistics) []evidence.Evidence {
	if stats == nil || stats.apiError != nil {
		return nil
	}
	const source = "cloudwatch:GetMetricStatistics"
	var facts []evidence.Evidence
	if stats.invocations != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.functionName, Field: "Invocations24h", Value: *stats.invocations, Confidence: evidence.Fact})
	}
	if stats.errors != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.functionName, Field: "Errors24h", Value: *stats.errors, Confidence: evidence.Fact})
	}
	if stats.throttles != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.functionName, Field: "Throttles24h", Value: *stats.throttles, Confidence: evidence.Fact})
	}
	if stats.durationMs != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.functionName, Field: "AvgDurationMs", Value: *stats.durationMs, Confidence: evidence.Fact})
	}
	return facts
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-0010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring every other applyAINarration
// in this codebase.
func (stats *functionStatistics) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		stats.aiSummaryUnavailable = "no evidence could be gathered"
		stats.aiRecommendationsUnavailable = "no evidence could be gathered"
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithNestedProgress(ctx, "Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})

	if result.summaryErr != "" {
		stats.aiSummaryUnavailable = result.summaryErr
	} else {
		stats.aiSummary = result.summary
	}
	if result.recommendedErr != "" {
		stats.aiRecommendationsUnavailable = result.recommendedErr
	} else {
		stats.aiRecommendations = result.recommendations
	}
}

func (f functionStatisticsFetcher) sumMetric(ctx context.Context, metricName string, start, end time.Time) (*float64, error) {
	out, err := f.getMetricStatistics(ctx, metricName, cwtypes.StatisticSum, start, end)
	if err != nil {
		return nil, err
	}
	var total float64
	var found bool
	for _, dp := range out.Datapoints {
		if dp.Sum != nil {
			total += *dp.Sum
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	return &total, nil
}

func (f functionStatisticsFetcher) averageMetric(ctx context.Context, metricName string, start, end time.Time) (*float64, error) {
	out, err := f.getMetricStatistics(ctx, metricName, cwtypes.StatisticAverage, start, end)
	if err != nil {
		return nil, err
	}
	var total float64
	var count int
	for _, dp := range out.Datapoints {
		if dp.Average != nil {
			total += *dp.Average
			count++
		}
	}
	if count == 0 {
		return nil, nil
	}
	mean := total / float64(count)
	return &mean, nil
}

func (f functionStatisticsFetcher) getMetricStatistics(ctx context.Context, metricName string, stat cwtypes.Statistic, start, end time.Time) (*cloudwatch.GetMetricStatisticsOutput, error) {
	return f.client.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/Lambda"),
		MetricName: aws.String(metricName),
		Dimensions: []cwtypes.Dimension{{Name: aws.String("FunctionName"), Value: aws.String(f.functionName)}},
		StartTime:  &start,
		EndTime:    &end,
		Period:     aws.Int32(lambdaMetricPeriodSeconds),
		Statistics: []cwtypes.Statistic{stat},
	})
}

func functionStatisticsViewer(data *functionStatistics, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	dataPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("Statistics for %s (last 24h)", data.functionName))
	if data.apiError != nil {
		dataPanel.SetBody("metrics unavailable: " + data.apiError.Error())
		return dataPanel
	}
	dataPanel.AddEntry("Invocations", formatFloatPtr(data.invocations, "%.0f"))
	dataPanel.AddEntry("Errors", formatFloatPtr(data.errors, "%.0f"))
	dataPanel.AddEntry("Throttles", formatFloatPtr(data.throttles, "%.0f"))
	dataPanel.AddEntry("Avg Duration (ms)", formatFloatPtr(data.durationMs, "%.1f"))

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", data.functionName))
	if data.aiSummary != "" {
		summaryPanel.SetBody(data.aiSummary)
	} else {
		reason := data.aiSummaryUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		summaryPanel.SetBody("summary unavailable: " + reason)
	}
	compound.AddViewer(summaryPanel)

	recommendationsPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Recommendations for %s (Recommendation — a human must apply these)", data.functionName))
	if data.aiRecommendations != "" {
		recommendationsPanel.SetBody(data.aiRecommendations)
	} else {
		reason := data.aiRecommendationsUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		recommendationsPanel.SetBody("recommendations unavailable: " + reason)
	}
	compound.AddViewer(recommendationsPanel)

	compound.AddViewer(dataPanel)
	return compound
}

func formatFloatPtr(v *float64, format string) string {
	if v == nil {
		return "no data"
	}
	return fmt.Sprintf(format, *v)
}
