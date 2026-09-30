package rds

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// maxEventsForNarration caps how many events feed the AI summary/
// recommendations prompt — a long --since window can return far more events
// than are useful to hand a local model in one prompt. The full event list
// is still shown in the table regardless of this cap.
const maxEventsForNarration = 20

// eventsAPI is the minimal client capability this file needs, letting
// tests substitute a fake instead of a real *rds.Client (ADR-0007).
type eventsAPI interface {
	DescribeEvents(ctx context.Context, params *rds.DescribeEventsInput, optFns ...func(*rds.Options)) (*rds.DescribeEventsOutput, error)
}

// dbEvent is one RDS lifecycle event — backups, failovers, restarts,
// low-storage warnings — sourced from RDS's own event stream (DescribeEvents),
// not CloudTrail. Many of these have no corresponding CloudTrail API call at
// all (e.g. an automated failover), which is exactly why this is a distinct
// data source from `ctl changes aws` rather than redundant with it.
type dbEvent struct {
	date       *time.Time
	message    *string
	categories []string
}

type dbEventListOutput struct {
	identifier string
	since      time.Duration
	events     []*dbEvent

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}

type dbEventListFetcher struct {
	client     eventsAPI
	identifier string
	since      time.Duration
}

func (f dbEventListFetcher) Fetch(ctx context.Context) (*dbEventListOutput, error) {
	viewer.SetProgress(ctx, fmt.Sprintf("Calling RDS DescribeEvents for %s...", f.identifier))
	durationMinutes := int32(f.since.Minutes())
	out, err := f.client.DescribeEvents(ctx, &rds.DescribeEventsInput{
		SourceIdentifier: &f.identifier,
		SourceType:       types.SourceTypeDbInstance,
		Duration:         &durationMinutes,
	})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	events := make([]*dbEvent, 0, len(out.Events))
	for _, e := range out.Events {
		categories := make([]string, len(e.EventCategories))
		copy(categories, e.EventCategories)
		events = append(events, &dbEvent{date: e.Date, message: e.Message, categories: categories})
	}
	if len(events) == 0 {
		return nil, ctlaws.NewErrorInfo(NoEventsFound(f.identifier), viewer.INFO, nil)
	}

	data := &dbEventListOutput{identifier: f.identifier, since: f.since, events: events}
	data.applyAINarration(ctx, ai.NewClientFromEnv(), dbEventListEvidence(data))
	return data, nil
}

// dbEventListEvidence converts already-fetched events into Fact-tagged
// evidence for Summarize/Recommend, capped at maxEventsForNarration so a
// long --since window doesn't blow out the prompt.
func dbEventListEvidence(data *dbEventListOutput) []evidence.Evidence {
	if data == nil || len(data.events) == 0 {
		return nil
	}
	const source = "rds:DescribeEvents"
	limit := len(data.events)
	if limit > maxEventsForNarration {
		limit = maxEventsForNarration
	}
	facts := make([]evidence.Evidence, 0, limit)
	for _, e := range data.events[:limit] {
		ts := "-"
		if e.date != nil {
			ts = e.date.UTC().Format(time.RFC3339)
		}
		message := "-"
		if e.message != nil {
			message = *e.message
		}
		value := ts
		if len(e.categories) > 0 {
			value += " [" + strings.Join(e.categories, ", ") + "]"
		}
		value += ": " + message
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: data.identifier, Field: "Event", Value: value, Confidence: evidence.Fact})
	}
	return facts
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-0010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring every other applyAINarration
// in this codebase.
func (data *dbEventListOutput) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		data.aiSummaryUnavailable = "no evidence could be gathered"
		data.aiRecommendationsUnavailable = "no evidence could be gathered"
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
		data.aiSummaryUnavailable = result.summaryErr
	} else {
		data.aiSummary = result.summary
	}
	if result.recommendedErr != "" {
		data.aiRecommendationsUnavailable = result.recommendedErr
	} else {
		data.aiRecommendations = result.recommendations
	}
}

var dbEventListTableHeader = viewer.Row{
	"Time",
	"Categories",
	"Message",
}

func dbEventListViewer(data *dbEventListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Events for %s (last %s)", data.identifier, data.since))
	tv.AddHeader(dbEventListTableHeader)
	for _, e := range data.events {
		ts := "-"
		if e.date != nil {
			ts = e.date.UTC().Format(time.RFC3339)
		}
		message := "-"
		if e.message != nil {
			message = *e.message
		}
		tv.AddRow(viewer.Row{ts, strings.Join(e.categories, ", "), message})
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the events below)", data.identifier))
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

	compound.AddViewer(tv)
	return compound
}
