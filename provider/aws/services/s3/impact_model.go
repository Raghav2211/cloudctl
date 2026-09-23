package s3

// policyMatch is one customer-managed IAM policy whose document was found
// (via deterministic ARN matching, never an AI judgment call) to reference
// the target bucket, plus the principals it's attached to.
type policyMatch struct {
	policyName string
	policyArn  string
	effect     string
	actions    []string
	principals []string
}

// bucketImpact is `ctl s3 impact <bucket>`'s output: the deterministic IAM
// cross-reference plus a Hypothesis-grade AI risk narration of it. aiSummary
// is additive, never a replacement for matches — if empty,
// aiSummaryUnavailable explains why (ADR-010), mirroring bucketDefinition
// and instanceDefinition.
type bucketImpact struct {
	bucketName string
	matches    []policyMatch

	aiSummary            string
	aiSummaryUnavailable string

	relationshipsSaved int
	relationshipsError string
}

func newBucketImpact(bucketName string) *bucketImpact {
	return &bucketImpact{bucketName: bucketName}
}

func (impact *bucketImpact) SetAISummary(summary string) *bucketImpact {
	impact.aiSummary = summary
	return impact
}

func (impact *bucketImpact) SetAISummaryUnavailable(reason string) *bucketImpact {
	impact.aiSummaryUnavailable = reason
	return impact
}
