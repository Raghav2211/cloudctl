package s3

import (
	"cloudctl/evidence"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// The format* helpers below turn a bucket-config API response into a
// human-readable value, returning ok=false when that dimension's fetch
// never produced data (either the call failed, or simply hasn't run).
// They're shared by bucketDefinitionEvidence (Fact-tagged evidence for
// Summarize, which omits a fact entirely on ok=false) and the panel-building
// code in viewer.go (which shows the underlying fetch error instead of
// omitting it — a rendered view should be honest about what's missing).

func formatPolicy(policy *s3.GetBucketPolicyOutput) (string, bool) {
	if policy == nil || policy.Policy == nil {
		return "", false
	}
	return *policy.Policy, true
}

func formatVersioning(version *s3.GetBucketVersioningOutput) (string, bool) {
	if version == nil {
		return "", false
	}
	status := string(version.Status)
	if status == "" {
		status = "Disabled"
	}
	return status, true
}

func formatTags(tags *s3.GetBucketTaggingOutput) (string, bool) {
	if tags == nil {
		return "", false
	}
	if len(tags.TagSet) == 0 {
		return "none", true
	}
	pairs := make([]string, 0, len(tags.TagSet))
	for _, t := range tags.TagSet {
		if t.Key != nil && t.Value != nil {
			pairs = append(pairs, fmt.Sprintf("%s=%s", *t.Key, *t.Value))
		}
	}
	return strings.Join(pairs, ", "), true
}

func formatEncryption(encryption *s3.GetBucketEncryptionOutput) (string, bool) {
	if encryption == nil || encryption.ServerSideEncryptionConfiguration == nil {
		return "", false
	}
	algos := make([]string, 0, len(encryption.ServerSideEncryptionConfiguration.Rules))
	for _, rule := range encryption.ServerSideEncryptionConfiguration.Rules {
		if rule.ApplyServerSideEncryptionByDefault != nil {
			algos = append(algos, string(rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm))
		}
	}
	if len(algos) == 0 {
		return "none", true
	}
	return strings.Join(algos, ", "), true
}

func formatLifecycle(lifecycle *s3.GetBucketLifecycleConfigurationOutput) (string, bool) {
	if lifecycle == nil {
		return "", false
	}
	if len(lifecycle.Rules) == 0 {
		return "no rules configured", true
	}
	enabled := 0
	for _, r := range lifecycle.Rules {
		if r.Status == types.ExpirationStatusEnabled {
			enabled++
		}
	}
	return fmt.Sprintf("%d rule(s), %d enabled", len(lifecycle.Rules), enabled), true
}

// bucketDefinitionEvidence converts whichever of the five bucket-config
// fetches succeeded into Fact-tagged evidence for Summarize. A dimension
// whose fetch failed is simply omitted — a partial fetch still produces
// partial, honest evidence rather than an all-or-nothing summary.
func bucketDefinitionEvidence(
	bucketName string,
	policy *s3.GetBucketPolicyOutput,
	version *s3.GetBucketVersioningOutput,
	tags *s3.GetBucketTaggingOutput,
	encryption *s3.GetBucketEncryptionOutput,
	lifecycle *s3.GetBucketLifecycleConfigurationOutput,
) []evidence.Evidence {
	facts := []evidence.Evidence{}

	if v, ok := formatPolicy(policy); ok {
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketPolicy", ResourceID: bucketName, Field: "BucketPolicy",
			Value: v, Confidence: evidence.Fact,
		})
	}
	if v, ok := formatVersioning(version); ok {
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketVersioning", ResourceID: bucketName, Field: "Versioning",
			Value: v, Confidence: evidence.Fact,
		})
	}
	if v, ok := formatTags(tags); ok {
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketTagging", ResourceID: bucketName, Field: "Tags",
			Value: v, Confidence: evidence.Fact,
		})
	}
	if v, ok := formatEncryption(encryption); ok {
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketEncryption", ResourceID: bucketName, Field: "Encryption",
			Value: v, Confidence: evidence.Fact,
		})
	}
	if v, ok := formatLifecycle(lifecycle); ok {
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketLifecycleConfiguration", ResourceID: bucketName, Field: "Lifecycle",
			Value: v, Confidence: evidence.Fact,
		})
	}

	return facts
}
