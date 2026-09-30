package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestBucketSecurityFindings_FlagsPublicPolicy(t *testing.T) {
	name := "my-bucket"
	def := &bucketDefinition{
		bucketName:   &name,
		policyStatus: &s3.GetBucketPolicyStatusOutput{PolicyStatus: &types.PolicyStatus{IsPublic: aws.Bool(true)}},
	}

	findings := bucketSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleS3PublicBucketPolicy {
		t.Fatalf("expected 1 %s finding, got %+v", ruleS3PublicBucketPolicy, findings)
	}
}

func TestBucketSecurityFindings_NoFindingWhenNotPublic(t *testing.T) {
	name := "my-bucket"
	def := &bucketDefinition{
		bucketName:   &name,
		policyStatus: &s3.GetBucketPolicyStatusOutput{PolicyStatus: &types.PolicyStatus{IsPublic: aws.Bool(false)}},
	}

	if findings := bucketSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a non-public bucket, got %+v", findings)
	}
}

func TestBucketSecurityFindings_NoFindingWhenPolicyStatusUnavailable(t *testing.T) {
	name := "my-bucket"
	def := &bucketDefinition{bucketName: &name, policyStatusAPIErr: assertErr("no policy")}

	if findings := bucketSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings when policy status couldn't be fetched, got %+v", findings)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
