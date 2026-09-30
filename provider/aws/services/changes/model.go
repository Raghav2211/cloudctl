package changes

import "time"

// change is one CloudTrail event, normalized to the fields the roadmap's
// change-timeline concept asks for: timestamp, change type (event name),
// source, actor, and the resources it touched.
type change struct {
	eventID     *string
	eventName   *string
	eventTime   *time.Time
	eventSource *string
	username    *string
	resources   []string
	// description is a short, human-readable sentence synthesized
	// deterministically from the fields above (see buildDescription in
	// fetcher.go) — CloudTrail gives no natural-language description of its
	// own beyond the raw event name, and this is never AI-generated.
	description string
}

type changeListOutput struct {
	changes []*change

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}
