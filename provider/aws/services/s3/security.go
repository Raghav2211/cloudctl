package s3

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// NewBucketSecurityCommandExecutor runs `ctl aws s3 security <bucket>`: the
// same definition fetch as `ctl aws s3 def`, evaluated against a small set
// of deterministic security rules (never AI-generated — see cloudctl/
// security's package doc).
func NewBucketSecurityCommandExecutor(cfg aws.Config, bucketName string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: bucketSecurityFetcher{cfg: cfg, bucketName: bucketName},
		Viewer:  security.Viewer,
	}
}

type bucketSecurityFetcher struct {
	cfg        aws.Config
	bucketName string
}

func (f bucketSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	def, err := NewBucketViewCommandExecutor(f.cfg, f.bucketName).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.bucketName, Findings: bucketSecurityFindings(def)}, nil
}

const ruleS3PublicBucketPolicy = "s3-public-bucket-policy"

// bucketSecurityFindings flags a bucket whose effective policy AWS itself
// has computed as public (via GetBucketPolicyStatus, which factors in
// Block Public Access and the policy's full effect — far more reliable
// than pattern-matching the raw policy JSON ourselves).
func bucketSecurityFindings(def *bucketDefinition) []security.Finding {
	if def == nil || def.policyStatus == nil || def.policyStatus.PolicyStatus == nil {
		return nil
	}
	isPublic := def.policyStatus.PolicyStatus.IsPublic
	if isPublic == nil || !*isPublic {
		return nil
	}
	id := derefStr(def.bucketName)
	return []security.Finding{{
		Rule:        ruleS3PublicBucketPolicy,
		Severity:    security.Critical,
		ResourceID:  id,
		Description: "AWS has determined this bucket's effective policy grants public access.",
		Remediation: "Review the bucket policy and Block Public Access settings; remove any statement granting access to Principal \"*\" unless public access is genuinely intended.",
		Evidence: []evidence.Evidence{
			{Source: "s3:GetBucketPolicyStatus", ResourceID: id, Field: "PolicyIsPublic", Value: true, Confidence: evidence.Fact},
		},
	}}
}
