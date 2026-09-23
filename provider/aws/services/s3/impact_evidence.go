package s3

import (
	"cloudctl/evidence"
	"fmt"
	"strings"
)

// derefStr returns "-" for a nil pointer instead of dereferencing it.
func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// bucketARN returns the S3 ARN for a bucket name. S3 ARNs are global (no
// region/account segment), unlike most other services'.
func bucketARN(bucketName string) string {
	return fmt.Sprintf("arn:aws:s3:::%s", bucketName)
}

// statementActionsForBucket is the one deterministic rule this feature
// relies on: which of a statement's actions, if any, plausibly grant access
// to this bucket? If the statement's Resource is the exact bucket ARN or the
// bucket's object ARN (bucket/*), every one of its actions genuinely applies
// to this bucket, whatever they are. If the Resource is a bare "*", only the
// S3-namespaced actions count — "*" is the standard AWS idiom for actions
// that don't support resource-level permissions at all (ec2:Describe*,
// logs:*, eks:*, ...), so a broad admin policy (e.g. an SSO permission set
// with hundreds of wildcard grants across every service) would otherwise
// report every unrelated action as if it were about this bucket. A nil/empty
// return means the statement doesn't grant access. This is a fixed rule, not
// a judgment call, so its result is Inference-tagged evidence, never
// Hypothesis.
func statementActionsForBucket(resources, actions []string, targetBucketARN string) []string {
	for _, r := range resources {
		if r == targetBucketARN || r == targetBucketARN+"/*" {
			return actions
		}
	}
	for _, r := range resources {
		if r != "*" {
			continue
		}
		var relevant []string
		for _, a := range actions {
			if a == "*" || strings.HasPrefix(a, "s3:") {
				relevant = append(relevant, a)
			}
		}
		return relevant
	}
	return nil
}

// toStringSlice normalizes an IAM policy document's Action/Resource field,
// which AWS serializes as either a single string or a JSON array of
// strings depending on how the policy was authored.
func toStringSlice(v any) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// dedupeStrings preserves first-seen order while dropping repeats.
// Terraform-generated policies routinely have one statement per
// user/resource pair, so a policy with many matching statements can easily
// repeat the same action or principal dozens of times — without this, the
// rendered actions/principals list balloons unreadably (found live: one
// real policy's Actions column rendered as a single ~17,000-character
// table row before this fix).
func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// bucketImpactEvidence converts the deterministic policy cross-reference
// into Inference-tagged evidence for Summarize — every fact here is derived
// via resourceMatchesBucket's fixed rule, not an AI judgment call.
func bucketImpactEvidence(impact *bucketImpact) []evidence.Evidence {
	var facts []evidence.Evidence
	for _, m := range impact.matches {
		facts = append(facts, evidence.Evidence{
			Source: "iam:cross-reference", ResourceID: impact.bucketName, Field: "PolicyGrant",
			Value:      fmt.Sprintf("policy=%s effect=%s actions=[%s] principals=[%s]", m.policyName, m.effect, strings.Join(m.actions, ","), strings.Join(m.principals, ",")),
			Confidence: evidence.Inference,
		})
	}
	return facts
}
