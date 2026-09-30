package changes

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

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

// maxChangesForNarration caps how many changes feed the AI summary/
// recommendations prompt — a long --since window can return far more
// changes than are useful to hand a local model in one prompt. The full
// change list is still shown in the table regardless of this cap.
const maxChangesForNarration = 20

// cloudTrailAPI is the minimal client capability this package needs,
// letting tests substitute a fake instead of a real *cloudtrail.Client
// (ADR-0007).
type cloudTrailAPI interface {
	LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error)
}

const maxResultsPerPage = 50

type changeListFetcher struct {
	client       cloudTrailAPI
	resourceName string // optional: filters to events referencing this resource
	eventName    string // optional: filters to only this event name
	// excludeEvents drops any event whose name is a key in this set (e.g.
	// {"AssumeRole": {}}) — always client-side, since CloudTrail's
	// LookupEvents has no negation/exclusion filter of its own.
	excludeEvents map[string]struct{}
	since         time.Duration
	limit         int32
}

// Fetch looks up CloudTrail management events in [now-since, now], paginating
// via NextToken until limit events (after filtering) are collected or
// CloudTrail runs out of pages. CloudTrail's own MaxResults caps at 50 per
// call, so a limit above that requires multiple calls — and filtering can
// require even more, since a page's events may be partly or wholly dropped.
//
// CloudTrail's LookupAttributes accepts only one attribute per call, so
// resourceName always wins the server-side filter slot when both it and
// eventName are set; eventName is then applied client-side alongside
// excludeEvents instead of being wasted.
func (f changeListFetcher) Fetch(ctx context.Context) (*changeListOutput, error) {
	end := time.Now()
	start := end.Add(-f.since)

	limit := f.limit
	if limit <= 0 {
		limit = maxResultsPerPage
	}

	input := &cloudtrail.LookupEventsInput{StartTime: &start, EndTime: &end}
	switch {
	case f.resourceName != "":
		resourceName := f.resourceName
		input.LookupAttributes = []types.LookupAttribute{
			{AttributeKey: types.LookupAttributeKeyResourceName, AttributeValue: &resourceName},
		}
	case f.eventName != "":
		eventName := f.eventName
		input.LookupAttributes = []types.LookupAttribute{
			{AttributeKey: types.LookupAttributeKeyEventName, AttributeValue: &eventName},
		}
	}
	needsClientSideEventNameFilter := f.eventName != "" && f.resourceName != ""
	filtering := needsClientSideEventNameFilter || len(f.excludeEvents) > 0

	var events []*change
	for {
		remaining := limit - int32(len(events))
		if remaining <= 0 {
			break
		}
		// When client-side filtering can drop events, request a full page
		// every time rather than exactly `remaining` — otherwise a
		// heavily-filtered window could take far more round-trips than
		// necessary to fill up to limit.
		pageSize := remaining
		if filtering || pageSize > maxResultsPerPage {
			pageSize = maxResultsPerPage
		}
		input.MaxResults = &pageSize

		out, err := f.client.LookupEvents(ctx, input)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, e := range out.Events {
			if needsClientSideEventNameFilter && (e.EventName == nil || *e.EventName != f.eventName) {
				continue
			}
			if e.EventName != nil {
				if _, excluded := f.excludeEvents[*e.EventName]; excluded {
					continue
				}
			}
			events = append(events, newChange(e))
			if int32(len(events)) >= limit {
				break
			}
		}
		if out.NextToken == nil || int32(len(events)) >= limit {
			break
		}
		input.NextToken = out.NextToken
	}

	if len(events) == 0 {
		return nil, ctlaws.NewErrorInfo(NoChangesFound(), viewer.INFO, nil)
	}

	data := &changeListOutput{changes: events}
	data.applyAINarration(ctx, ai.NewClientFromEnv(), changeListEvidence(data))
	return data, nil
}

// changeListEvidence converts already-fetched changes into Fact-tagged
// evidence for Summarize/Recommend, capped at maxChangesForNarration so a
// long --since window doesn't blow out the prompt.
func changeListEvidence(data *changeListOutput) []evidence.Evidence {
	if data == nil || len(data.changes) == 0 {
		return nil
	}
	const source = "cloudtrail:LookupEvents"
	limit := len(data.changes)
	if limit > maxChangesForNarration {
		limit = maxChangesForNarration
	}
	facts := make([]evidence.Evidence, 0, limit)
	for _, c := range data.changes[:limit] {
		facts = append(facts, evidence.Evidence{Source: source, ResourceID: derefStr(c.eventName), Field: "Change", Value: c.description, Confidence: evidence.Fact})
	}
	return facts
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-0010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring every other applyAINarration
// in this codebase.
func (data *changeListOutput) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		data.aiSummaryUnavailable = "no evidence could be gathered"
		data.aiRecommendationsUnavailable = "no evidence could be gathered"
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
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

func newChange(e types.Event) *change {
	resourceNames := make([]string, 0, len(e.Resources))
	for _, r := range e.Resources {
		if r.ResourceName != nil {
			resourceNames = append(resourceNames, *r.ResourceName)
		}
	}
	c := &change{
		eventID:     e.EventId,
		eventName:   e.EventName,
		eventTime:   e.EventTime,
		eventSource: e.EventSource,
		username:    e.Username,
		resources:   resourceNames,
	}
	c.description = buildDescription(c)
	return c
}

// buildDescription synthesizes a short, human-readable sentence from an
// event's already-known fields — the roadmap's explicit "short description"
// requirement. This is deterministic string composition, never invented:
// CloudTrail gives no natural-language description of its own beyond the
// raw event name.
func buildDescription(c *change) string {
	who := "someone"
	if c.username != nil && *c.username != "" {
		who = *c.username
	}
	what := "an unrecorded action"
	if c.eventName != nil {
		what = *c.eventName
	}
	on := ""
	if len(c.resources) > 0 {
		on = " on " + strings.Join(c.resources, ", ")
	}
	via := ""
	if c.eventSource != nil {
		via = " via " + shortSource(*c.eventSource)
	}
	return fmt.Sprintf("%s called %s%s%s", who, what, on, via)
}

// shortSource strips the ".amazonaws.com" suffix AWS event sources always
// carry (e.g. "ec2.amazonaws.com" -> "ec2") for a more compact description.
func shortSource(source string) string {
	return strings.TrimSuffix(source, ".amazonaws.com")
}
