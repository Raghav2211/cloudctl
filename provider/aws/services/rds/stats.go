package rds

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

// dbStatistics is `ctl aws rds stats <identifier>`'s output: the CloudWatch
// metrics that matter most for an RDS instance/cluster's operational health,
// over the last 24 hours.
type dbStatistics struct {
	identifier string

	cpuPercent     *float64 // mean of hourly averages
	connectionsAvg *float64 // mean of hourly averages
	freeStorageGB  *float64 // most recent (last) datapoint — a point-in-time gauge, not summed/averaged
	readLatencyMs  *float64 // mean of hourly averages
	writeLatencyMs *float64 // mean of hourly averages

	apiError *ctlaws.ErrorInfo

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}

type dbStatisticsFetcher struct {
	client     getMetricStatisticsAPI
	identifier string
}

const rdsMetricWindow = 24 * time.Hour
const rdsMetricPeriodSeconds = 3600

func (f dbStatisticsFetcher) Fetch(ctx context.Context) (*dbStatistics, error) {
	end := time.Now()
	start := end.Add(-rdsMetricWindow)

	stats := &dbStatistics{identifier: f.identifier}

	viewer.SetProgress(ctx, fmt.Sprintf("Fetching CloudWatch statistics for %s...", f.identifier))
	cpu, err := f.averageMetric(ctx, "CPUUtilization", start, end)
	if err != nil {
		stats.apiError = ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		return stats, nil
	}
	stats.cpuPercent = cpu

	if v, err := f.averageMetric(ctx, "DatabaseConnections", start, end); err == nil {
		stats.connectionsAvg = v
	}
	if v, err := f.lastMetric(ctx, "FreeStorageSpace", start, end); err == nil && v != nil {
		gb := *v / (1024 * 1024 * 1024)
		stats.freeStorageGB = &gb
	}
	if v, err := f.averageMetric(ctx, "ReadLatency", start, end); err == nil {
		stats.readLatencyMs = scaleMs(v)
	}
	if v, err := f.averageMetric(ctx, "WriteLatency", start, end); err == nil {
		stats.writeLatencyMs = scaleMs(v)
	}

	stats.applyAINarration(ctx, ai.NewClientFromEnv(), dbStatisticsEvidence(stats))

	return stats, nil
}

// dbStatisticsEvidence converts already-fetched metrics into Fact-tagged
// evidence for Summarize/Recommend — only metrics that actually returned
// data contribute a fact, mirroring every other evidence builder in this
// codebase's partial-failure tolerance.
func dbStatisticsEvidence(stats *dbStatistics) []evidence.Evidence {
	if stats == nil || stats.apiError != nil {
		return nil
	}
	const source = "cloudwatch:GetMetricStatistics"
	var facts []evidence.Evidence
	if stats.cpuPercent != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.identifier, Field: "CPUPercent24h", Value: *stats.cpuPercent, Confidence: evidence.Fact})
	}
	if stats.connectionsAvg != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.identifier, Field: "AvgConnections24h", Value: *stats.connectionsAvg, Confidence: evidence.Fact})
	}
	if stats.freeStorageGB != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.identifier, Field: "FreeStorageGB", Value: *stats.freeStorageGB, Confidence: evidence.Fact})
	}
	if stats.readLatencyMs != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.identifier, Field: "ReadLatencyMs24h", Value: *stats.readLatencyMs, Confidence: evidence.Fact})
	}
	if stats.writeLatencyMs != nil {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: stats.identifier, Field: "WriteLatencyMs24h", Value: *stats.writeLatencyMs, Confidence: evidence.Fact})
	}
	return facts
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-0010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring every other applyAINarration
// in this codebase.
func (stats *dbStatistics) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
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

// scaleMs converts RDS's Read/WriteLatency (reported in seconds) to
// milliseconds, the more readable unit for typical single-digit-ms values.
func scaleMs(seconds *float64) *float64 {
	if seconds == nil {
		return nil
	}
	ms := *seconds * 1000
	return &ms
}

func (f dbStatisticsFetcher) averageMetric(ctx context.Context, metricName string, start, end time.Time) (*float64, error) {
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

// lastMetric returns the most recent datapoint's average — used for gauges
// like FreeStorageSpace where a mean-over-window is less meaningful than
// "what is it right now".
func (f dbStatisticsFetcher) lastMetric(ctx context.Context, metricName string, start, end time.Time) (*float64, error) {
	out, err := f.getMetricStatistics(ctx, metricName, cwtypes.StatisticAverage, start, end)
	if err != nil {
		return nil, err
	}
	var latest *cwtypes.Datapoint
	for i := range out.Datapoints {
		dp := out.Datapoints[i]
		if dp.Timestamp == nil || dp.Average == nil {
			continue
		}
		if latest == nil || dp.Timestamp.After(*latest.Timestamp) {
			latest = &out.Datapoints[i]
		}
	}
	if latest == nil {
		return nil, nil
	}
	return latest.Average, nil
}

func (f dbStatisticsFetcher) getMetricStatistics(ctx context.Context, metricName string, stat cwtypes.Statistic, start, end time.Time) (*cloudwatch.GetMetricStatisticsOutput, error) {
	return f.client.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/RDS"),
		MetricName: aws.String(metricName),
		Dimensions: []cwtypes.Dimension{{Name: aws.String("DBInstanceIdentifier"), Value: aws.String(f.identifier)}},
		StartTime:  &start,
		EndTime:    &end,
		Period:     aws.Int32(rdsMetricPeriodSeconds),
		Statistics: []cwtypes.Statistic{stat},
	})
}

func dbStatisticsViewer(data *dbStatistics, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	dataPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("Statistics for %s (last 24h)", data.identifier))
	if data.apiError != nil {
		dataPanel.SetBody("metrics unavailable: " + data.apiError.Error())
		return dataPanel
	}
	dataPanel.AddEntry("CPU (%)", formatFloatPtr(data.cpuPercent, "%.1f"))
	dataPanel.AddEntry("Connections (avg)", formatFloatPtr(data.connectionsAvg, "%.1f"))
	dataPanel.AddEntry("Free Storage (GB)", formatFloatPtr(data.freeStorageGB, "%.2f"))
	dataPanel.AddEntry("Read Latency (ms)", formatFloatPtr(data.readLatencyMs, "%.2f"))
	dataPanel.AddEntry("Write Latency (ms)", formatFloatPtr(data.writeLatencyMs, "%.2f"))

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", data.identifier))
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

	recommendationsPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Recommendations for %s (Recommendation — a human must apply these)", data.identifier))
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
