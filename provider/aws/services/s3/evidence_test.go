package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestBucketDefinitionEvidence_AllDimensionsPresent(t *testing.T) {
	facts := bucketDefinitionEvidence(
		"my-bucket",
		&s3.GetBucketPolicyOutput{Policy: aws.String(`{"Statement":[]}`)},
		&s3.GetBucketVersioningOutput{Status: types.BucketVersioningStatusEnabled},
		&s3.GetBucketTaggingOutput{TagSet: []types.Tag{{Key: aws.String("team"), Value: aws.String("payments")}}},
		&s3.GetBucketEncryptionOutput{ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{{ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{SSEAlgorithm: types.ServerSideEncryptionAwsKms}}},
		}},
		&s3.GetBucketLifecycleConfigurationOutput{Rules: []types.LifecycleRule{{Status: types.ExpirationStatusEnabled}}},
	)

	if len(facts) != 5 {
		t.Fatalf("expected 5 evidence entries, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Confidence != "FACT" {
			t.Errorf("expected all evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
		if f.ResourceID != "my-bucket" {
			t.Errorf("expected ResourceID my-bucket, got %q", f.ResourceID)
		}
	}
}

func TestBucketDefinitionEvidence_PartialFailure_OnlySuccessfulDimensions(t *testing.T) {
	// policy and tags fetches failed (nil), everything else succeeded.
	facts := bucketDefinitionEvidence(
		"my-bucket",
		nil,
		&s3.GetBucketVersioningOutput{Status: ""}, // never configured
		nil,
		&s3.GetBucketEncryptionOutput{ServerSideEncryptionConfiguration: nil},
		&s3.GetBucketLifecycleConfigurationOutput{Rules: nil},
	)

	if len(facts) != 2 {
		t.Fatalf("expected 2 evidence entries (versioning, lifecycle), got %d: %+v", len(facts), facts)
	}

	var sawVersioning, sawLifecycle bool
	for _, f := range facts {
		switch f.Field {
		case "Versioning":
			sawVersioning = true
			if f.Value != "Disabled" {
				t.Errorf("expected empty versioning status to render as Disabled, got %v", f.Value)
			}
		case "Lifecycle":
			sawLifecycle = true
			if f.Value != "no rules configured" {
				t.Errorf("expected no lifecycle rules to render clearly, got %v", f.Value)
			}
		}
	}
	if !sawVersioning || !sawLifecycle {
		t.Fatalf("expected both Versioning and Lifecycle facts, got %+v", facts)
	}
}

func TestBucketDefinitionEvidence_AllFailed_NoEvidence(t *testing.T) {
	facts := bucketDefinitionEvidence("my-bucket", nil, nil, nil, nil, nil)
	if len(facts) != 0 {
		t.Fatalf("expected no evidence when every fetch failed, got %+v", facts)
	}
}
