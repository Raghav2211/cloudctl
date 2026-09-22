package s3

import (
	"cloudctl/evidence"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// bucketDefinitionEvidence converts whichever of the five bucket-config
// fetches succeeded into Fact-tagged evidence for Summarize. Each parameter
// is nil if that dimension's fetch failed — a partial fetch still produces
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

	if policy != nil && policy.Policy != nil {
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketPolicy", ResourceID: bucketName, Field: "BucketPolicy",
			Value: *policy.Policy, Confidence: evidence.Fact,
		})
	}

	if version != nil {
		status := string(version.Status)
		if status == "" {
			status = "Disabled"
		}
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketVersioning", ResourceID: bucketName, Field: "Versioning",
			Value: status, Confidence: evidence.Fact,
		})
	}

	if tags != nil {
		value := "none"
		if len(tags.TagSet) > 0 {
			pairs := make([]string, 0, len(tags.TagSet))
			for _, t := range tags.TagSet {
				if t.Key != nil && t.Value != nil {
					pairs = append(pairs, fmt.Sprintf("%s=%s", *t.Key, *t.Value))
				}
			}
			value = strings.Join(pairs, ", ")
		}
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketTagging", ResourceID: bucketName, Field: "Tags",
			Value: value, Confidence: evidence.Fact,
		})
	}

	if encryption != nil && encryption.ServerSideEncryptionConfiguration != nil {
		algos := make([]string, 0, len(encryption.ServerSideEncryptionConfiguration.Rules))
		for _, rule := range encryption.ServerSideEncryptionConfiguration.Rules {
			if rule.ApplyServerSideEncryptionByDefault != nil {
				algos = append(algos, string(rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm))
			}
		}
		value := "none"
		if len(algos) > 0 {
			value = strings.Join(algos, ", ")
		}
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketEncryption", ResourceID: bucketName, Field: "Encryption",
			Value: value, Confidence: evidence.Fact,
		})
	}

	if lifecycle != nil {
		value := "no rules configured"
		if len(lifecycle.Rules) > 0 {
			enabled := 0
			for _, r := range lifecycle.Rules {
				if r.Status == types.ExpirationStatusEnabled {
					enabled++
				}
			}
			value = fmt.Sprintf("%d rule(s), %d enabled", len(lifecycle.Rules), enabled)
		}
		facts = append(facts, evidence.Evidence{
			Source: "s3:GetBucketLifecycleConfiguration", ResourceID: bucketName, Field: "Lifecycle",
			Value: value, Confidence: evidence.Fact,
		})
	}

	return facts
}
