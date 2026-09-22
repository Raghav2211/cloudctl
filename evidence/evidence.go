// Package evidence defines the epistemic-status types AI-narrated commands
// use to keep AI conclusions traceable back to the facts that grounded them.
package evidence

// Confidence is the epistemic status of a piece of evidence: how certain we
// are that it's true, not how severe a problem it represents. That's a
// separate, orthogonal axis already covered by viewer.ErrorType/
// aws.ErrorInfo — don't conflate the two.
type Confidence string

const (
	// Fact is a direct API field, with no interpretation applied.
	Fact Confidence = "FACT"
	// Inference is derived via a deterministic rule from one or more Facts.
	Inference Confidence = "INFERENCE"
	// Correlation notes that two facts co-occur; it makes no causal claim.
	Correlation Confidence = "CORRELATION"
	// Hypothesis is an AI-generated explanation, unverified by any
	// deterministic code path.
	Hypothesis Confidence = "HYPOTHESIS"
	// Recommendation is a suggested action. A human must apply it — nothing
	// in this codebase ever applies one automatically.
	Recommendation Confidence = "RECOMMENDATION"
)

// Evidence is one piece of grounding for a Fact, Inference, or Hypothesis.
//
// Only code paths that never call an LLM may construct an Evidence tagged
// Fact or Inference. Hypothesis and Recommendation are reserved for
// AI-generated output, and must be tagged by the code that calls the model,
// never self-reported by the model itself — models are unreliable judges of
// their own certainty.
type Evidence struct {
	Source     string // e.g. "s3:GetBucketPolicy"
	ResourceID string
	Field      string // e.g. "Policy.Statement[0].Effect"
	Value      any
	Confidence Confidence
}
